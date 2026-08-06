import { createWriteStream } from "node:fs";
import { mkdir, rm } from "node:fs/promises";
import { basename, join } from "node:path";
import { randomUUID } from "node:crypto";
import { pipeline } from "node:stream/promises";
import type { FastifyRequest } from "fastify";
import type { AppConfig } from "../config.js";
import { HttpError } from "../errors.js";
import type { UploadedReplay } from "../service/replay-analysis-service.js";

function safeFileName(fileName: string): string {
  return basename(fileName).replace(/[^a-zA-Z0-9._-]/g, "_").slice(-180) || "replay.rofl";
}

export async function saveReplayUpload(request: FastifyRequest, config: AppConfig): Promise<UploadedReplay> {
  const requestId = randomUUID();
  const workDirectory = join(config.workDirectory, requestId);
  await mkdir(workDirectory, { recursive: true });
  try {
    const part = await request.file({ limits: { files: 1, fileSize: config.maxUploadBytes } });
    if (!part) throw new HttpError(400, "MISSING_REPLAY", "multipart/form-data의 replay 파일이 필요합니다.");
    if (part.fieldname !== "replay") throw new HttpError(400, "INVALID_FILE_FIELD", "파일 필드 이름은 replay여야 합니다.");
    if (!part.filename.toLowerCase().endsWith(".rofl")) throw new HttpError(400, "INVALID_FILE_TYPE", ".rofl 파일만 업로드할 수 있습니다.");
    const fileName = safeFileName(part.filename);
    const path = join(workDirectory, fileName);
    await pipeline(part.file, createWriteStream(path, { flags: "wx" }));
    if (part.file.truncated) throw new HttpError(413, "FILE_TOO_LARGE", "ROFL 파일이 업로드 제한을 초과했습니다.");
    return { requestId, fileName, path, workDirectory };
  } catch (error) {
    await rm(workDirectory, { recursive: true, force: true }).catch(() => undefined);
    throw error;
  }
}
