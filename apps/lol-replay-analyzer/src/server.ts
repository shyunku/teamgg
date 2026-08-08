import { mkdir } from "node:fs/promises";
import { buildApp } from "./app.js";
import { loadConfig } from "./config.js";

const config = loadConfig();
await Promise.all([
  mkdir(config.workDirectory, { recursive: true }),
  mkdir(config.roflCacheDirectory, { recursive: true }),
]);

const app = await buildApp(config);
app.log.info(
  {
    event: "service.configured",
    host: config.host,
    port: config.port,
    logLevel: config.logLevel,
    maxUploadBytes: config.maxUploadBytes,
    maxConcurrentAnalyses: config.maxConcurrentAnalyses,
    analysisTimeoutMs: config.analysisTimeoutMs,
    workDirectory: config.workDirectory,
    roflCacheDirectory: config.roflCacheDirectory,
    keepArtifacts: config.keepArtifacts,
    aiConfigured: Boolean(config.openaiApiKey && config.openaiModel),
    model: config.openaiModel ?? null,
    teamggIntegrationConfigured: Boolean(config.teamggApiBaseUrl && config.replayAnalyzerSharedSecret),
  },
  "replay analyzer configured",
);
const close = async (signal: string) => {
  app.log.info({ signal }, "shutting down");
  await app.close();
  process.exit(0);
};
process.once("SIGINT", () => void close("SIGINT"));
process.once("SIGTERM", () => void close("SIGTERM"));

try {
  await app.listen({ host: config.host, port: config.port });
} catch (error) {
  app.log.fatal(error);
  process.exit(1);
}
