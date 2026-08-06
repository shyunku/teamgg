import { mkdir } from "node:fs/promises";
import { buildApp } from "./app.js";
import { loadConfig } from "./config.js";

const config = loadConfig();
await Promise.all([
  mkdir(config.workDirectory, { recursive: true }),
  mkdir(config.roflCacheDirectory, { recursive: true }),
]);

const app = await buildApp(config);
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
