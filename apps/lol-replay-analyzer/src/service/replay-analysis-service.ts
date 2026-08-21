import { mkdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { AppConfig } from "../config.js";
import { buildPromptAssets } from "../ai/prompt.js";
import type { AnalysisOptions, AnalysisResult, ProgressReporter, ProgressUpdate, ReplayDigest, TextDeltaReporter } from "../domain.js";
import { HttpError, throwIfAborted } from "../errors.js";
import { buildDigest } from "../rofl/digest.js";
import { inspectRofl } from "../rofl/metadata.js";
import { decodeReplay } from "../rofl/parser.js";
import { Semaphore } from "../semaphore.js";
import type { ReplayAnalyzer } from "../ai/analyzer.js";

export interface UploadedReplay { requestId: string; fileName: string; path: string; workDirectory: string; }
export interface AnalysisRunOptions { retainArtifacts?: boolean; }
const clampProgress = (value: number | undefined): number => Math.min(1, Math.max(0, value ?? 0));
export function normalizeAnalysisProgress(update: ProgressUpdate): number {
  const local = clampProgress(update.progress);
  switch (update.stage) {
    case "queued": return 0.02;
    case "upload-complete": return 0.05;
    case "replay-inspection": return 0.08;
    case "executable-resolution": return 0.1 + local * 0.15;
    case "decoder-preparation": return 0.25 + local * 0.1;
    case "replay-decoding": return 0.35 + local * 0.35;
    case "digest-building": return 0.72;
    case "prompt-building": return 0.8;
    case "ai-analysis": return 0.86;
    case "complete": return 1;
  }
}
export function createMonotonicProgressReporter(report: ProgressReporter): ProgressReporter {
  let previous = 0;
  return (update) => {
    const progress = Math.max(previous, normalizeAnalysisProgress(update));
    previous = progress;
    report({ ...update, progress });
  };
}
async function writePromptArtifacts(decodedPath: string, digest: ReplayDigest, options: AnalysisOptions): Promise<void> {
  const root = join(decodedPath, "process", "prompt-assets"); await mkdir(root, { recursive: true });
  await Promise.all(buildPromptAssets(digest, options).map((asset) => writeFile(join(root, asset.fileName), `${JSON.stringify(asset.data, null, 2)}\n`, "utf8")));
}
async function writeResultArtifact(decodedPath: string, analysis: string): Promise<void> { await writeFile(join(decodedPath, "process", "result.md"), `${analysis}\n`, "utf8"); }
export class ReplayAnalysisService {
  private readonly semaphore: Semaphore;
  constructor(private readonly config: AppConfig, private readonly analyzer?: ReplayAnalyzer) { this.semaphore = new Semaphore(config.maxConcurrentAnalyses); }
  get status() { return { running: this.semaphore.running, pending: this.semaphore.pending }; }
  async analyze(upload: UploadedReplay, options: AnalysisOptions, signal: AbortSignal, report: ProgressReporter, onDelta?: TextDeltaReporter, runOptions: AnalysisRunOptions = {}): Promise<AnalysisResult> {
    let release: (() => void) | undefined; let decodedPath: string | undefined;
    const retainArtifacts = runOptions.retainArtifacts === true || this.config.keepArtifacts;
    const reportOverall = createMonotonicProgressReporter(report);
    try {
      if (!this.analyzer) throw new HttpError(503, "AI_NOT_CONFIGURED", "AI 분석 설정이 완료되지 않았습니다.");
      reportOverall({ stage: "queued", message: "분석 작업 대기열에 등록되었습니다.", detail: this.status }); release = await this.semaphore.acquire(signal); throwIfAborted(signal);
      reportOverall({ stage: "upload-complete", message: "업로드가 완료되어 분석을 시작합니다." });
      reportOverall({ stage: "replay-inspection", message: "ROFL 헤더와 경기 종료 통계를 검사하고 있습니다." }); const inspection = await inspectRofl(upload.path, signal);
      const decoded = await decodeReplay(upload.path, join(upload.workDirectory, "decoded"), this.config, signal, reportOverall); decodedPath = decoded.path; throwIfAborted(signal);
      reportOverall({ stage: "digest-building", message: "원본을 정제하고 검증된 분석 데이터를 구성하고 있습니다." }); const digest = await buildDigest(inspection, decoded);
      reportOverall({ stage: "prompt-building", message: "검증된 사건 근거로 AI 입력을 구성하고 있습니다." });
      await writePromptArtifacts(decoded.path, digest, options);
      reportOverall({ stage: "ai-analysis", message: "AI가 승패 요인과 개인별 피드백을 작성하고 있습니다." });
      const analysis = onDelta ? await this.analyzer.analyzeStream(digest, options, signal, onDelta) : await this.analyzer.analyze(digest, options, signal);
      await writeResultArtifact(decoded.path, analysis); reportOverall({ stage: "complete", message: "리플레이 분석이 완료되었습니다.", progress: 1 });
      return { requestId: upload.requestId, digest, analysis, model: this.analyzer.model, ...(retainArtifacts ? { retainedArtifacts: { root: upload.workDirectory, refined: join(decoded.path, "refined"), process: join(decoded.path, "process") } } : {}) };
    } finally { release?.(); if (!retainArtifacts) await rm(upload.workDirectory, { recursive: true, force: true }).catch(() => undefined); else if (!decodedPath) await rm(upload.workDirectory, { recursive: true, force: true }).catch(() => undefined); }
  }
}
