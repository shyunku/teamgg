import "dotenv/config";
import { resolve } from "node:path";
import { z } from "zod";

function parseBoolean(value: string | undefined, fallback: boolean): boolean {
  if (value == null || value.trim() === "") return fallback;
  return ["1", "true", "yes", "on"].includes(value.trim().toLowerCase());
}

function parseDurationMs(value: string | undefined, fallback: number): number {
  if (value == null || value.trim() === "") return fallback;
  const match = value.trim().match(/^(\d+(?:\.\d+)?)(ms|s|m|h)$/i);
  if (!match) throw new Error(`Invalid duration: ${value}. Use ms, s, m, or h.`);
  const amount = Number(match[1]);
  const unit = match[2]?.toLowerCase();
  const multiplier = unit === "ms" ? 1 : unit === "s" ? 1_000 : unit === "m" ? 60_000 : 3_600_000;
  return Math.round(amount * multiplier);
}

function emptyStringToUndefined(value: unknown): unknown {
  return typeof value === "string" && value.trim() === "" ? undefined : value;
}

const environmentSchema = z.object({
  NODE_ENV: z.enum(["development", "test", "production"]).default("development"),
  HOST: z.string().default("0.0.0.0"),
  PORT: z.coerce.number().int().min(1).max(65_535).default(7720),
  LOG_LEVEL: z.string().default("info"),
  MAX_UPLOAD_MB: z.coerce.number().positive().default(250),
  MAX_CONCURRENT_ANALYSES: z.coerce.number().int().positive().max(16).default(1),
  WORK_DIR: z.string().default("./.work"),
  ROFL_CACHE_DIR: z.string().default("./.rofl-cache"),
  LOL_EXECUTABLE_PATH: z.string().optional(),
  LOL_DOWNLOAD_REGION: z.string().default("KR"),
  OPENAI_API_KEY: z.string().optional(),
  OPENAI_MODEL: z.string().optional(),
  OPENAI_BASE_URL: z.preprocess(emptyStringToUndefined, z.string().url().optional()),
  OPENAI_REASONING_EFFORT: z.preprocess(
    emptyStringToUndefined,
    z.enum(["none", "low", "medium", "high", "xhigh", "max"]).optional(),
  ),
  OPENAI_MAX_OUTPUT_TOKENS: z.coerce.number().int().positive().max(32_000).default(6_000),
  CORS_ORIGINS: z.string().default("http://localhost:8080"),
  TEAMGG_API_BASE_URL: z.preprocess(emptyStringToUndefined, z.string().url().optional()),
  REPLAY_ANALYZER_SHARED_SECRET: z.string().optional(),
});

export type AppConfig = ReturnType<typeof loadConfig>;

export function loadConfig(environment: NodeJS.ProcessEnv = process.env) {
  const env = environmentSchema.parse(environment);
  return {
    nodeEnv: env.NODE_ENV,
    host: env.HOST,
    port: env.PORT,
    logLevel: env.LOG_LEVEL,
    maxUploadBytes: Math.floor(env.MAX_UPLOAD_MB * 1024 * 1024),
    maxConcurrentAnalyses: env.MAX_CONCURRENT_ANALYSES,
    analysisTimeoutMs: parseDurationMs(environment.ANALYSIS_TIMEOUT, 20 * 60_000),
    workDirectory: resolve(env.WORK_DIR),
    roflCacheDirectory: resolve(env.ROFL_CACHE_DIR),
    keepArtifacts: parseBoolean(environment.KEEP_ARTIFACTS, false),
    executablePath: env.LOL_EXECUTABLE_PATH?.trim() ? resolve(env.LOL_EXECUTABLE_PATH) : undefined,
    downloadRegion: env.LOL_DOWNLOAD_REGION,
    openaiApiKey: env.OPENAI_API_KEY?.trim() || undefined,
    openaiModel: env.OPENAI_MODEL?.trim() || undefined,
    openaiBaseUrl: env.OPENAI_BASE_URL?.trim() || undefined,
    openaiReasoningEffort: env.OPENAI_REASONING_EFFORT,
    openaiMaxOutputTokens: env.OPENAI_MAX_OUTPUT_TOKENS,
    corsOrigins: env.CORS_ORIGINS.split(",").map((value) => value.trim()).filter(Boolean),
    teamggApiBaseUrl: env.TEAMGG_API_BASE_URL?.trim() || undefined,
    replayAnalyzerSharedSecret: env.REPLAY_ANALYZER_SHARED_SECRET?.trim() || undefined,
  };
}
