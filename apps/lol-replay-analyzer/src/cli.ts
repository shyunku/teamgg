import { randomUUID } from "node:crypto";
import { mkdir, stat, writeFile } from "node:fs/promises";
import { basename, extname, resolve } from "node:path";
import { OpenAIReplayAnalyzer } from "./ai/openai-analyzer.js";
import { loadConfig } from "./config.js";
import type { AnalysisOptions, ProgressUpdate } from "./domain.js";
import { errorMessage, HttpError } from "./errors.js";
import { ReplayAnalysisService } from "./service/replay-analysis-service.js";

interface CliOptions {
  replayPath: string;
  language: string;
  focusPlayer?: string;
  direct: boolean;
  json: boolean;
  outputPath?: string;
  keepArtifacts: boolean;
}

function usage(): string {
  return `사용법:
  npm run analyze:rofl -- <replay.rofl> [옵션]

옵션:
  --direct                    AI 응답을 완료 후 한 번에 출력 (기본: 실시간 스트림)
  --json                      direct 모드에서 digest를 포함한 전체 결과를 JSON으로 출력
  --language <언어>           결과 언어 (기본: Korean)
  --focus-player <닉네임#태그> 특정 플레이어 피드백을 더 자세히 요청
  --output <파일>             최종 Markdown 분석을 파일로 저장
  --keep-artifacts            디코딩·정제·프롬프트·응답 작업 폴더를 보존
  --help                      도움말 출력

예시:
  npm run analyze:rofl -- "C:\\replays\\KR-123.rofl"
  npm run analyze:rofl -- "C:\\replays\\KR-123.rofl" --direct
  npm run analyze:rofl -- "C:\\replays\\KR-123.rofl" --focus-player "닉네임#KR1" --output result.md`;
}

function parseArguments(args: string[]): CliOptions {
  if (args.includes("--help") || args.includes("-h")) {
    process.stdout.write(`${usage()}\n`);
    process.exit(0);
  }
  let replayPath: string | undefined;
  let language = "Korean";
  let focusPlayer: string | undefined;
  let outputPath: string | undefined;
  let direct = false;
  let json = false;
  let keepArtifacts = false;

  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index];
    if (argument === "--direct") { direct = true; continue; }
    if (argument === "--json") { json = true; direct = true; continue; }
    if (argument === "--keep-artifacts") { keepArtifacts = true; continue; }
    if (["--language", "--focus-player", "--output"].includes(argument ?? "")) {
      const next = args[++index];
      if (!next) throw new HttpError(400, "INVALID_ARGUMENT", `${argument} 뒤에 값이 필요합니다.`);
      if (argument === "--language") language = next;
      if (argument === "--focus-player") focusPlayer = next;
      if (argument === "--output") outputPath = resolve(next);
      continue;
    }
    if (argument?.startsWith("--")) throw new HttpError(400, "INVALID_ARGUMENT", `알 수 없는 옵션입니다: ${argument}`);
    if (replayPath) throw new HttpError(400, "INVALID_ARGUMENT", "ROFL 파일은 하나만 지정할 수 있습니다.");
    replayPath = argument;
  }

  if (!replayPath) throw new HttpError(400, "MISSING_REPLAY", `ROFL 파일 경로가 필요합니다.\n\n${usage()}`);
  return {
    replayPath: resolve(replayPath),
    language,
    direct,
    json,
    keepArtifacts,
    ...(focusPlayer ? { focusPlayer } : {}),
    ...(outputPath ? { outputPath } : {}),
  };
}

function createProgressReporter() {
  let previous = "";
  return (update: ProgressUpdate) => {
    const percentage = update.progress === undefined ? "" : ` ${Math.round(update.progress * 100)}%`;
    const key = `${update.stage}:${percentage}:${update.message}`;
    if (key === previous) return;
    previous = key;
    process.stderr.write(`[${update.stage}]${percentage} ${update.message}\n`);
  };
}

async function main(): Promise<void> {
  const options = parseArguments(process.argv.slice(2));
  if (extname(options.replayPath).toLowerCase() !== ".rofl") {
    throw new HttpError(400, "INVALID_FILE_TYPE", ".rofl 파일만 분석할 수 있습니다.");
  }
  const replayStat = await stat(options.replayPath).catch(() => null);
  if (!replayStat?.isFile()) throw new HttpError(404, "REPLAY_NOT_FOUND", `ROFL 파일을 찾을 수 없습니다: ${options.replayPath}`);

  const config = loadConfig();
  const analyzer = new OpenAIReplayAnalyzer(config);
  const service = new ReplayAnalysisService(config, analyzer);
  const requestId = randomUUID();
  const workDirectory = resolve(config.workDirectory, `cli-${requestId}`);
  await Promise.all([mkdir(workDirectory, { recursive: true }), mkdir(config.roflCacheDirectory, { recursive: true })]);

  const controller = new AbortController();
  const timeout = setTimeout(
    () => controller.abort(new HttpError(504, "ANALYSIS_TIMEOUT", "리플레이 분석 제한 시간을 초과했습니다.")),
    config.analysisTimeoutMs,
  );
  timeout.unref();
  const onInterrupt = () => controller.abort(new Error("사용자가 분석을 중단했습니다."));
  process.once("SIGINT", onInterrupt);

  const analysisOptions: AnalysisOptions = {
    language: options.language,
    ...(options.focusPlayer ? { focusPlayer: options.focusPlayer } : {}),
  };
  process.stderr.write(`ROFL 분석 시작: ${options.replayPath}\n`);
  process.stderr.write(`모델: ${analyzer.model} / 모드: ${options.direct ? "direct" : "stream"}\n\n`);

  try {
    const result = await service.analyze(
      { requestId, fileName: basename(options.replayPath), path: options.replayPath, workDirectory },
      analysisOptions,
      controller.signal,
      createProgressReporter(),
      options.direct ? undefined : (delta) => process.stdout.write(delta),
      { retainArtifacts: options.keepArtifacts },
    );
    if (options.direct) process.stdout.write(options.json ? `${JSON.stringify(result, null, 2)}\n` : `${result.analysis}\n`);
    else process.stdout.write("\n");
    if (options.keepArtifacts && result.retainedArtifacts) process.stderr.write(`작업 폴더 보존: ${result.retainedArtifacts.root}\n`);
    if (options.outputPath) {
      await writeFile(options.outputPath, `${result.analysis}\n`, "utf8");
      process.stderr.write(`\n결과 저장: ${options.outputPath}\n`);
    }
  } finally {
    clearTimeout(timeout);
    process.removeListener("SIGINT", onInterrupt);
  }
}

main().catch((error: unknown) => {
  process.stderr.write(`\n분석 실패: ${errorMessage(error)}\n`);
  if (error instanceof Error && error.cause) process.stderr.write(`원인: ${errorMessage(error.cause)}\n`);
  process.exitCode = 1;
});
