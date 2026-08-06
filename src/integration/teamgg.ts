import { createHmac, timingSafeEqual } from "node:crypto";
import { errorMessage } from "../errors.js";
import type { AnalysisResult, ProgressUpdate } from "../domain.js";

export function validateUploadTicket(secret: string, ticket: string, expectedJobId: string, now = new Date()): void {
  if (!secret || !ticket) throw new Error("리플레이 업로드 티켓이 필요합니다.");
  const parts = ticket.split(".");
  if (parts.length !== 2) throw new Error("리플레이 업로드 티켓이 올바르지 않습니다.");
  const [payload, encodedSignature] = parts;
  if (!payload || !encodedSignature) throw new Error("리플레이 업로드 티켓이 올바르지 않습니다.");
  const signature = Buffer.from(encodedSignature, "base64url");
  const expectedSignature = createHmac("sha256", secret).update(payload).digest();
  if (signature.length !== expectedSignature.length || !timingSafeEqual(signature, expectedSignature)) {
    throw new Error("리플레이 업로드 티켓 서명이 올바르지 않습니다.");
  }
  const decoded = Buffer.from(payload, "base64url").toString("utf8");
  const separator = decoded.lastIndexOf(".");
  const jobId = decoded.slice(0, separator);
  const expiresAt = Number(decoded.slice(separator + 1));
  if (separator <= 0 || jobId !== expectedJobId || !Number.isFinite(expiresAt)) {
    throw new Error("리플레이 업로드 티켓이 작업과 일치하지 않습니다.");
  }
  if (now.getTime() > expiresAt * 1_000) throw new Error("리플레이 업로드 티켓이 만료되었습니다.");
}

type JobUpdate = {
  requestId?: string;
  status?: "uploading" | "analyzing" | "completed" | "failed";
  stage?: string;
  progress?: number;
  analysis?: string;
  model?: string;
  error?: string;
};

export class TeamggJobReporter {
  private pending = Promise.resolve();
  private lastProgressAt = 0;
  private lastProgress = -1;
  private queuedProgress: JobUpdate | undefined;
  private progressTimer: NodeJS.Timeout | undefined;

  constructor(
    private readonly apiBaseUrl: string,
    private readonly sharedSecret: string,
    private readonly jobId: string,
    private readonly onBackgroundError: (error: unknown) => void,
  ) {}

  private async send(update: JobUpdate, maxAttempts = 3): Promise<void> {
    const url = `${this.apiBaseUrl.replace(/\/+$/, "")}/v1/internal/replay-analysis/${encodeURIComponent(this.jobId)}`;
    let lastError: unknown;
    for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
      try {
        const response = await fetch(url, {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
            "X-Replay-Analyzer-Secret": this.sharedSecret,
          },
          body: JSON.stringify(update),
          signal: AbortSignal.timeout(10_000),
        });
        if (!response.ok) throw new Error(`team.gg callback failed (${response.status}): ${await response.text()}`);
        return;
      } catch (error) {
        lastError = error;
        if (attempt < maxAttempts - 1) {
          await new Promise((resolve) => setTimeout(resolve, Math.min(5_000, 500 * 2 ** attempt)));
        }
      }
    }
    throw lastError;
  }

  async uploading(): Promise<void> {
    await this.send({ status: "uploading", stage: "리플레이 업로드 중", progress: 1 });
  }

  async started(requestId: string): Promise<void> {
    await this.send({ requestId, status: "analyzing", stage: "업로드 완료 · 분석 작업 준비 중", progress: 15 });
  }

  private enqueue(update: JobUpdate): void {
    this.pending = this.pending
      .then(() => this.send(update))
      .catch((error) => this.onBackgroundError(error));
  }

  private flushProgress(): void {
    if (this.progressTimer) {
      clearTimeout(this.progressTimer);
      this.progressTimer = undefined;
    }
    const update = this.queuedProgress;
    this.queuedProgress = undefined;
    if (!update) return;
    this.lastProgressAt = Date.now();
    this.enqueue(update);
  }

  progress(update: ProgressUpdate): void {
    const percent = Math.max(15, Math.min(99, Math.round((update.progress ?? 0) * 100)));
    if (percent <= this.lastProgress) return;
    this.lastProgress = percent;
    this.queuedProgress = { status: "analyzing", stage: update.message, progress: percent };
    const now = Date.now();
    const wait = Math.max(0, 1_000 - (now - this.lastProgressAt));
    if (wait === 0) {
      this.flushProgress();
    } else if (!this.progressTimer) {
      this.progressTimer = setTimeout(() => this.flushProgress(), wait);
      this.progressTimer.unref();
    }
  }

  async completed(result: AnalysisResult): Promise<void> {
    this.flushProgress();
    await this.pending;
    await this.send({
      requestId: result.requestId,
      status: "completed",
      stage: "분석 완료",
      progress: 100,
      analysis: result.analysis,
      model: result.model,
    }, 8);
  }

  async failed(error: unknown): Promise<void> {
    this.flushProgress();
    await this.pending;
    await this.send({ status: "failed", stage: "분석 실패", error: errorMessage(error) }, 8);
  }
}
