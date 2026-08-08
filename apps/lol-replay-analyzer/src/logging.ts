import type { FastifyRequest } from "fastify";
import type { AppConfig } from "./config.js";
import type { ProgressUpdate } from "./domain.js";

export const SERVICE_NAME = "teamgg-lol-replay-analyzer";
export const SERVICE_VERSION = "0.1.0";

export function buildLoggerOptions(config: Pick<AppConfig, "logLevel" | "nodeEnv">) {
  return {
    level: config.logLevel,
    base: {
      service: SERVICE_NAME,
      version: SERVICE_VERSION,
      environment: config.nodeEnv,
    },
    timestamp: () => `,"timestamp":"${new Date().toISOString()}"`,
    formatters: {
      level: (label: string) => ({ level: label }),
    },
    redact: {
      paths: [
        "req.headers.authorization",
        "req.headers.cookie",
        'req.headers["x-replay-upload-ticket"]',
      ],
      censor: "[REDACTED]",
    },
  };
}

export interface AnalysisLogContext {
  analysisId: string;
  fileName: string;
  mode: "direct" | "stream" | "shared-stream";
  startedAt: number;
  jobId?: string;
}

export function createAnalysisProgressLogger(request: FastifyRequest, context: AnalysisLogContext) {
  let previousStage: ProgressUpdate["stage"] | undefined;
  let previousPercent = -1;

  return (update: ProgressUpdate): void => {
    const progressPercent = Math.round(Math.min(1, Math.max(0, update.progress ?? 0)) * 100);
    const stageChanged = update.stage !== previousStage;
    const advancedEnough = progressPercent >= previousPercent + 5;
    const completed = update.stage === "complete" || progressPercent === 100;
    if (!stageChanged && !advancedEnough && !completed) return;

    previousStage = update.stage;
    previousPercent = Math.max(previousPercent, progressPercent);
    request.log.info(
      {
        event: "replay.analysis.progress",
        analysisId: context.analysisId,
        fileName: context.fileName,
        mode: context.mode,
        ...(context.jobId ? { jobId: context.jobId } : {}),
        stage: update.stage,
        progressPercent,
        elapsedMs: Date.now() - context.startedAt,
        ...(update.detail ? { detail: update.detail } : {}),
      },
      update.message,
    );
  };
}
