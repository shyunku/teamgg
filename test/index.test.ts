import assert from "node:assert/strict";
import { createHash, createHmac } from "node:crypto";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { inspectRofl } from "../src/rofl/metadata.js";
import { Semaphore } from "../src/semaphore.js";
import { buildApp, resolveCorsOrigin } from "../src/app.js";
import { loadConfig } from "../src/config.js";
import { buildCandidates, calculateGoldSwing, objectiveImpact } from "../src/rofl/event-dossiers.js";
import { normalizeAnalysisProgress } from "../src/service/replay-analysis-service.js";
import { initializeSse } from "../src/http/sse.js";
import { TeamggJobReporter, validateUploadTicket } from "../src/integration/teamgg.js";

function syntheticRofl(): Buffer {
  const header = Buffer.alloc(32);
  header.write("RIOT", 0, "ascii");
  header.writeUInt16LE(2, 4);
  const version = "16.15.801.3452";
  header.writeUInt8(Buffer.byteLength(version), 14);
  header.write(version, 15, "utf8");
  const metadata = Buffer.from(JSON.stringify({
    gameId: 123,
    gameLength: 1_800_000,
    statsJson: JSON.stringify([{ RIOT_ID_GAME_NAME: "tester", RIOT_ID_TAG_LINE: "KR1", TEAM: "100", WIN: "Win" }]),
  }));
  const footer = Buffer.alloc(4);
  footer.writeUInt32LE(metadata.length);
  return Buffer.concat([header, Buffer.alloc(256), metadata, footer]);
}

test("ROFL footer inspection extracts version and post-game stats", async () => {
  const directory = await mkdtemp(join(tmpdir(), "teamgg-rofl-"));
  const path = join(directory, "test.rofl");
  const buffer = syntheticRofl();
  try {
    await writeFile(path, buffer);
    const result = await inspectRofl(path, new AbortController().signal);
    assert.equal(result.gameVersion, "16.15.801.3452");
    assert.equal(result.patch, "16.15");
    assert.equal(result.stats.length, 1);
    assert.equal(result.sha256, createHash("sha256").update(buffer).digest("hex"));
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("semaphore queues work above its capacity", async () => {
  const semaphore = new Semaphore(1);
  const signal = new AbortController().signal;
  const first = await semaphore.acquire(signal);
  let secondAcquired = false;
  const secondPromise = semaphore.acquire(signal).then((release) => { secondAcquired = true; return release; });
  await Promise.resolve();
  assert.equal(secondAcquired, false);
  assert.equal(semaphore.pending, 1);
  first();
  const second = await secondPromise;
  assert.equal(secondAcquired, true);
  second();
});

test("health endpoint works without an AI key", async () => {
  const config = loadConfig({
    NODE_ENV: "test",
    WORK_DIR: "./.test-work",
    ROFL_CACHE_DIR: "./.test-cache",
    OPENAI_BASE_URL: "",
    OPENAI_REASONING_EFFORT: "",
  });
  assert.equal(config.openaiReasoningEffort, undefined);
  const app = await buildApp(config);
  try {
    const response = await app.inject({ method: "GET", url: "/health" });
    assert.equal(response.statusCode, 200);
    assert.equal(response.json().ai.configured, false);
  } finally {
    await app.close();
  }
});

test("streaming responses only expose configured CORS origins", () => {
  const allowedOrigins = ["http://localhost:8080", "https://teamgg.kr"];
  assert.equal(resolveCorsOrigin("https://teamgg.kr", allowedOrigins), "https://teamgg.kr");
  assert.equal(resolveCorsOrigin("https://example.com", allowedOrigins), undefined);
  assert.equal(resolveCorsOrigin("https://example.com", ["*"]), "*");
  assert.equal(resolveCorsOrigin(undefined, allowedOrigins), undefined);
});

test("hijacked SSE responses include the resolved CORS origin", () => {
  const headers = new Map<string, string>();
  const response = {
    statusCode: 0,
    setHeader(name: string, value: string) {
      headers.set(name.toLowerCase(), value);
      return response;
    },
    flushHeaders() {},
  };

  initializeSse(response as never, "https://teamgg.kr");

  assert.equal(response.statusCode, 200);
  assert.equal(headers.get("access-control-allow-origin"), "https://teamgg.kr");
  assert.equal(headers.get("vary"), "Origin");
  assert.equal(headers.get("x-accel-buffering"), "no");
});

test("shared replay uploads allow the ticket header in CORS preflight", async () => {
  const config = loadConfig({
    NODE_ENV: "test",
    WORK_DIR: "./.test-work",
    ROFL_CACHE_DIR: "./.test-cache",
    CORS_ORIGINS: "http://localhost:8080",
    TEAMGG_API_BASE_URL: "http://localhost:7713",
    REPLAY_ANALYZER_SHARED_SECRET: "shared-secret",
  });
  const app = await buildApp(config);
  try {
    const response = await app.inject({
      method: "OPTIONS",
      url: "/v1/replays/jobs/job-1/upload/stream",
      headers: {
        origin: "http://localhost:8080",
        "access-control-request-method": "POST",
        "access-control-request-headers": "x-replay-upload-ticket,content-type",
      },
    });
    assert.equal(response.statusCode, 204);
    assert.match(response.headers["access-control-allow-headers"] ?? "", /X-Replay-Upload-Ticket/i);
  } finally {
    await app.close();
  }
});

test("team.gg upload tickets bind the job and expiration", () => {
  const now = new Date("2027-01-15T00:00:00Z");
  const payload = Buffer.from(`job-1.${Math.floor(now.getTime() / 1_000) + 60}`).toString("base64url");
  const signature = createHmac("sha256", "shared-secret").update(payload).digest("base64url");
  const ticket = `${payload}.${signature}`;

  assert.doesNotThrow(() => validateUploadTicket("shared-secret", ticket, "job-1", now));
  assert.throws(() => validateUploadTicket("shared-secret", ticket, "job-2", now));
  assert.throws(() => validateUploadTicket("shared-secret", ticket, "job-1", new Date(now.getTime() + 120_000)));
});

test("team.gg progress callbacks coalesce rapid updates", async () => {
  const originalFetch = globalThis.fetch;
  const updates: Array<Record<string, unknown>> = [];
  globalThis.fetch = async (_input, init) => {
    updates.push(JSON.parse(String(init?.body)) as Record<string, unknown>);
    return new Response(null, { status: 204 });
  };
  try {
    const errors: unknown[] = [];
    const reporter = new TeamggJobReporter("https://api.teamgg.test", "shared-secret", "job-1", (error) => errors.push(error));
    reporter.progress({ stage: "replay-inspection", message: "검사 중", progress: 0.2 });
    reporter.progress({ stage: "replay-decoding", message: "디코딩 중", progress: 0.3 });
    reporter.progress({ stage: "digest-building", message: "정제 중", progress: 0.4 });
    await new Promise((resolve) => setTimeout(resolve, 25));
    assert.equal(updates.filter((update) => update.status === "analyzing").length, 1);
    await reporter.failed(new Error("test failure"));
    assert.equal(updates.filter((update) => update.status === "analyzing").length, 2);
    assert.equal(updates.at(-2)?.progress, 40);
    assert.equal(updates.at(-1)?.status, "failed");
    assert.deepEqual(errors, []);
  } finally {
    globalThis.fetch = originalFetch;
  }
});
test("event impact weights objectives and gold lead reversals independently", () => {
  assert.equal(objectiveImpact("baron-killed", null), 500);
  assert.equal(objectiveImpact("dragon-killed", "fire"), 300);
  assert.equal(objectiveImpact("turret-destroyed", "outer"), 175);
  const timeline = [
    { t: 90, team100: 10_000, team200: 9_000, diff: 1_000 },
    { t: 130, team100: 11_000, team200: 12_500, diff: -1_500 },
  ];
  const swing = calculateGoldSwing(timeline, 100, 110);
  assert.equal(swing?.leadReversed, true);
  assert.equal(swing?.delta, -2_500);
  const [candidate] = buildCandidates([{ t: 100, victimId: "p6", killerId: "p1" }], [], timeline);
  assert.equal(candidate?.baseScore, 100);
  assert.equal(candidate?.score, 600);
});

test("event impact remains factual when a gold timeline is unavailable", () => {
  const [candidate] = buildCandidates(
    [{ t: 100, victimId: "p6", killerId: "p1" }],
    [{ t: 130, type: "dragon-killed", lane: null, tier: "fire", defendingTeamId: null, killerId: "p1" }],
  );
  assert.equal(candidate?.goldSwing, null);
  assert.equal(candidate?.baseScore, 400);
  assert.equal(candidate?.score, 400);
});

test("analysis stages map to monotonic overall progress", () => {
  const progress = [
    normalizeAnalysisProgress({ stage: "queued", message: "" }),
    normalizeAnalysisProgress({ stage: "upload-complete", message: "" }),
    normalizeAnalysisProgress({ stage: "replay-inspection", message: "" }),
    normalizeAnalysisProgress({ stage: "executable-resolution", message: "", progress: 1 }),
    normalizeAnalysisProgress({ stage: "decoder-preparation", message: "", progress: 1 }),
    normalizeAnalysisProgress({ stage: "replay-decoding", message: "", progress: 1 }),
    normalizeAnalysisProgress({ stage: "digest-building", message: "" }),
    normalizeAnalysisProgress({ stage: "prompt-building", message: "" }),
    normalizeAnalysisProgress({ stage: "ai-analysis", message: "" }),
    normalizeAnalysisProgress({ stage: "complete", message: "" }),
  ];
  assert.deepEqual(progress, [0.02, 0.05, 0.08, 0.25, 0.35, 0.7, 0.72, 0.8, 0.86, 1]);
  assert.equal(progress.every((value, index) => index === 0 || value >= (progress[index - 1] ?? 0)), true);
});
