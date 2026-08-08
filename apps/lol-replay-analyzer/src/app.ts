import Fastify, { type FastifyInstance } from "fastify";
import cors from "@fastify/cors";
import multipart from "@fastify/multipart";
import { z } from "zod";
import type { AppConfig } from "./config.js";
import { HttpError, errorMessage } from "./errors.js";
import { OpenAIReplayAnalyzer } from "./ai/openai-analyzer.js";
import type { ReplayAnalyzer } from "./ai/analyzer.js";
import { ReplayAnalysisService } from "./service/replay-analysis-service.js";
import { saveReplayUpload } from "./http/upload.js";
import { initializeSse, writeSse } from "./http/sse.js";
import { TeamggJobReporter, validateUploadTicket } from "./integration/teamgg.js";
import { buildLoggerOptions, createAnalysisProgressLogger, SERVICE_VERSION, type AnalysisLogContext } from "./logging.js";

const optionsSchema = z.object({ language: z.string().trim().min(2).max(40).default("Korean"), focusPlayer: z.string().trim().min(1).max(100).optional(), keepArtifacts: z.enum(["true", "false", "1", "0"]).optional() });
function requestOptions(query: unknown) {
  const parsed = optionsSchema.safeParse(query);
  if (!parsed.success) throw new HttpError(400, "INVALID_OPTIONS", "language, focusPlayer 또는 keepArtifacts 값이 올바르지 않습니다.");
  return { analysisOptions: parsed.data.focusPlayer ? { language: parsed.data.language, focusPlayer: parsed.data.focusPlayer } : { language: parsed.data.language }, retainArtifacts: parsed.data.keepArtifacts === "true" || parsed.data.keepArtifacts === "1" };
}
function requestController(request: { raw: NodeJS.EventEmitter }, timeoutMs: number) {
  const controller = new AbortController(); const onAborted = () => controller.abort(new HttpError(499, "CLIENT_ABORTED", "클라이언트가 요청을 종료했습니다."));
  request.raw.once("aborted", onAborted); const timeout = setTimeout(() => controller.abort(new HttpError(504, "ANALYSIS_TIMEOUT", "리플레이 분석 제한 시간을 초과했습니다.")), timeoutMs); timeout.unref();
  return { signal: controller.signal, dispose() { clearTimeout(timeout); request.raw.removeListener("aborted", onAborted); } };
}
function configuredAnalyzer(config: AppConfig): ReplayAnalyzer | undefined { return config.openaiApiKey && config.openaiModel ? new OpenAIReplayAnalyzer(config) : undefined; }
export function resolveCorsOrigin(origin: string | undefined, allowedOrigins: string[]): string | undefined {
  if (!origin) return undefined;
  if (allowedOrigins.includes("*")) return "*";
  return allowedOrigins.includes(origin) ? origin : undefined;
}
export async function buildApp(config: AppConfig, analyzer?: ReplayAnalyzer): Promise<FastifyInstance> {
  const app = Fastify({ logger: buildLoggerOptions(config), bodyLimit: config.maxUploadBytes }); const selectedAnalyzer = analyzer ?? configuredAnalyzer(config); const service = new ReplayAnalysisService(config, selectedAnalyzer);
  await app.register(cors, {
    origin: config.corsOrigins.length === 0 ? false : config.corsOrigins,
    methods: ["GET", "POST", "OPTIONS"],
    allowedHeaders: ["Accept", "Content-Type", "X-Replay-Upload-Ticket"],
  });
  await app.register(multipart, { limits: { files: 1, fields: 4, fileSize: config.maxUploadBytes } });
  app.get("/health", async () => ({ status: "ok", version: SERVICE_VERSION, isProduction: config.nodeEnv === "production", ai: { configured: Boolean(selectedAnalyzer), model: selectedAnalyzer?.model ?? null }, analysis: service.status }));
  app.post("/v1/replays/analyze", async (request, reply) => {
    const { analysisOptions, retainArtifacts } = requestOptions(request.query); const upload = await saveReplayUpload(request, config); const lifecycle = requestController(request, config.analysisTimeoutMs);
    const logContext: AnalysisLogContext = { analysisId: upload.requestId, fileName: upload.fileName, mode: "direct", startedAt: Date.now() };
    const logProgress = createAnalysisProgressLogger(request, logContext);
    request.log.info({ event: "replay.analysis.started", ...logContext, retainArtifacts }, "replay analysis started");
    try {
      const result = await service.analyze(upload, analysisOptions, lifecycle.signal, logProgress, undefined, { retainArtifacts });
      request.log.info({ event: "replay.analysis.completed", ...logContext, durationMs: Date.now() - logContext.startedAt, model: result.model, artifactsRetained: Boolean(result.retainedArtifacts) }, "replay analysis completed");
      return await reply.send(result);
    } catch (error) {
      request.log.error({ err: error, event: "replay.analysis.failed", ...logContext, durationMs: Date.now() - logContext.startedAt }, "replay analysis failed");
      throw error;
    } finally { lifecycle.dispose(); }
  });
  app.post("/v1/replays/analyze/stream", async (request, reply) => {
    const { analysisOptions, retainArtifacts } = requestOptions(request.query); const upload = await saveReplayUpload(request, config); const lifecycle = requestController(request, config.analysisTimeoutMs);
    const logContext: AnalysisLogContext = { analysisId: upload.requestId, fileName: upload.fileName, mode: "stream", startedAt: Date.now() };
    const logProgress = createAnalysisProgressLogger(request, logContext);
    request.log.info({ event: "replay.analysis.started", ...logContext, retainArtifacts }, "streaming replay analysis started");
    const allowOrigin = resolveCorsOrigin(request.headers.origin, config.corsOrigins);
    reply.hijack(); initializeSse(reply.raw, allowOrigin); writeSse(reply.raw, "started", { requestId: upload.requestId, fileName: upload.fileName });
    const keepAlive = setInterval(() => { if (!reply.raw.destroyed && !reply.raw.writableEnded) reply.raw.write(": keep-alive\n\n"); }, 15_000); keepAlive.unref();
    try {
      const result = await service.analyze(upload, analysisOptions, lifecycle.signal, (progress) => { logProgress(progress); writeSse(reply.raw, "progress", progress); }, (delta) => writeSse(reply.raw, "analysis_delta", { delta }), { retainArtifacts });
      request.log.info({ event: "replay.analysis.completed", ...logContext, durationMs: Date.now() - logContext.startedAt, model: result.model, artifactsRetained: Boolean(result.retainedArtifacts) }, "streaming replay analysis completed");
      writeSse(reply.raw, "result", result);
    }
    catch (error) { request.log.error({ err: error, event: "replay.analysis.failed", ...logContext, durationMs: Date.now() - logContext.startedAt }, "streaming replay analysis failed"); writeSse(reply.raw, "error", { code: error instanceof HttpError ? error.code : "ANALYSIS_FAILED", message: errorMessage(error) }); }
    finally { clearInterval(keepAlive); lifecycle.dispose(); if (!reply.raw.writableEnded) reply.raw.end(); }
  });
  app.post<{ Params: { jobId: string }; Querystring: { ticket?: string; language?: string; focusPlayer?: string; keepArtifacts?: string } }>("/v1/replays/jobs/:jobId/upload/stream", async (request, reply) => {
    if (!config.teamggApiBaseUrl || !config.replayAnalyzerSharedSecret) {
      throw new HttpError(503, "TEAMGG_INTEGRATION_DISABLED", "team.gg 공유 분석 연동이 설정되지 않았습니다.");
    }
    try {
      const headerTicket = request.headers["x-replay-upload-ticket"];
      const ticket = (Array.isArray(headerTicket) ? headerTicket[0] : headerTicket) ?? request.query.ticket ?? "";
      validateUploadTicket(config.replayAnalyzerSharedSecret, ticket, request.params.jobId);
    } catch (error) {
      throw new HttpError(401, "INVALID_UPLOAD_TICKET", errorMessage(error));
    }
    const reporter = new TeamggJobReporter(
      config.teamggApiBaseUrl,
      config.replayAnalyzerSharedSecret,
      request.params.jobId,
      (error) => request.log.warn({ err: error, jobId: request.params.jobId }, "replay progress callback failed"),
    );
    let parsedOptions: ReturnType<typeof requestOptions>;
    try {
      parsedOptions = requestOptions(request.query);
    } catch (error) {
      await reporter.failed(error).catch((callbackError) => request.log.error({ err: callbackError }, "replay failure callback failed"));
      throw error;
    }
    const { analysisOptions, retainArtifacts } = parsedOptions;
    await reporter.uploading();
    let upload;
    try {
      upload = await saveReplayUpload(request, config);
      await reporter.started(upload.requestId);
    } catch (error) {
      await reporter.failed(error).catch((callbackError) => request.log.error({ err: callbackError }, "replay failure callback failed"));
      throw error;
    }
    const logContext: AnalysisLogContext = { analysisId: upload.requestId, fileName: upload.fileName, mode: "shared-stream", startedAt: Date.now(), jobId: request.params.jobId };
    const logProgress = createAnalysisProgressLogger(request, logContext);
    request.log.info({ event: "replay.analysis.started", ...logContext, retainArtifacts }, "shared streaming replay analysis started");
    const lifecycle = requestController(request, config.analysisTimeoutMs);
    const allowOrigin = resolveCorsOrigin(request.headers.origin, config.corsOrigins);
    reply.hijack(); initializeSse(reply.raw, allowOrigin); writeSse(reply.raw, "started", { requestId: upload.requestId, fileName: upload.fileName, jobId: request.params.jobId });
    const keepAlive = setInterval(() => { if (!reply.raw.destroyed && !reply.raw.writableEnded) reply.raw.write(": keep-alive\n\n"); }, 15_000); keepAlive.unref();
    try {
      const result = await service.analyze(upload, analysisOptions, lifecycle.signal, (progress) => { logProgress(progress); reporter.progress(progress); writeSse(reply.raw, "progress", progress); }, (delta) => writeSse(reply.raw, "analysis_delta", { delta }), { retainArtifacts });
      await reporter.completed(result);
      request.log.info({ event: "replay.analysis.completed", ...logContext, durationMs: Date.now() - logContext.startedAt, model: result.model, artifactsRetained: Boolean(result.retainedArtifacts) }, "shared streaming replay analysis completed");
      writeSse(reply.raw, "result", { ...result, jobId: request.params.jobId });
    } catch (error) {
      request.log.error({ err: error, event: "replay.analysis.failed", ...logContext, durationMs: Date.now() - logContext.startedAt }, "shared streaming replay analysis failed");
      await reporter.failed(error).catch((callbackError) => request.log.error({ err: callbackError }, "replay failure callback failed"));
      writeSse(reply.raw, "error", { code: error instanceof HttpError ? error.code : "ANALYSIS_FAILED", message: errorMessage(error) });
    } finally {
      clearInterval(keepAlive); lifecycle.dispose(); if (!reply.raw.writableEnded) reply.raw.end();
    }
  });
  app.setErrorHandler((error, request, reply) => { request.log.error({ err: error }, "request failed"); const genericStatus = typeof error === "object" && error !== null && "statusCode" in error && typeof error.statusCode === "number" ? error.statusCode : 500; const statusCode = error instanceof HttpError ? error.statusCode : genericStatus; const code = error instanceof HttpError ? error.code : statusCode === 413 ? "FILE_TOO_LARGE" : "INTERNAL_ERROR"; const message = error instanceof Error && (error instanceof HttpError || statusCode < 500) ? error.message : "서버 내부 오류가 발생했습니다."; void reply.status(statusCode).send({ error: { code, message } }); });
  return app;
}
