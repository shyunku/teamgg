import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { open } from "node:fs/promises";
import { basename } from "node:path";
import { HttpError, throwIfAborted } from "../errors.js";

const MAX_METADATA_BYTES = 32 * 1024 * 1024;

export interface RoflInspection {
  fileName: string;
  fileSize: number;
  sha256: string;
  gameVersion: string;
  patch: string;
  metadata: Record<string, unknown>;
  stats: Record<string, unknown>[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

async function hashFile(path: string, signal: AbortSignal): Promise<string> {
  const hash = createHash("sha256");
  for await (const chunk of createReadStream(path, { signal })) hash.update(chunk as Buffer);
  return hash.digest("hex");
}

export async function inspectRofl(path: string, signal: AbortSignal): Promise<RoflInspection> {
  throwIfAborted(signal);
  const handle = await open(path, "r");
  try {
    const stat = await handle.stat();
    if (stat.size < 32) throw new HttpError(400, "INVALID_ROFL", "ROFL 파일이 너무 작습니다.");

    const header = Buffer.alloc(256);
    const headerRead = await handle.read(header, 0, header.length, 0);
    const head = header.subarray(0, headerRead.bytesRead);
    if (head.subarray(0, 4).toString("ascii") !== "RIOT") {
      throw new HttpError(400, "INVALID_ROFL", "ROFL2 파일 헤더가 아닙니다.");
    }
    const formatVersion = head.readUInt16LE(4);
    if (formatVersion !== 2) {
      throw new HttpError(400, "UNSUPPORTED_ROFL", `지원하지 않는 ROFL 형식 버전입니다: ${formatVersion}`);
    }
    const versionLength = head.readUInt8(14);
    if (15 + versionLength > head.length) throw new HttpError(400, "INVALID_ROFL", "게임 버전 헤더가 손상되었습니다.");
    const gameVersion = head.subarray(15, 15 + versionLength).toString("utf8");
    const patch = gameVersion.match(/^(\d+\.\d+)/)?.[1];
    if (!patch) throw new HttpError(400, "INVALID_ROFL", `게임 패치를 판별할 수 없습니다: ${gameVersion}`);

    const footer = Buffer.alloc(4);
    await handle.read(footer, 0, 4, stat.size - 4);
    const metadataLength = footer.readUInt32LE(0);
    if (metadataLength === 0 || metadataLength > MAX_METADATA_BYTES || metadataLength > stat.size - 4) {
      throw new HttpError(400, "INVALID_ROFL", "ROFL 메타데이터 길이가 올바르지 않습니다.");
    }
    const bytes = Buffer.alloc(metadataLength);
    await handle.read(bytes, 0, metadataLength, stat.size - 4 - metadataLength);
    const parsed: unknown = JSON.parse(bytes.toString("utf8"));
    if (!isRecord(parsed)) throw new HttpError(400, "INVALID_ROFL", "ROFL 메타데이터 형식이 올바르지 않습니다.");
    const statsValue = typeof parsed.statsJson === "string" ? JSON.parse(parsed.statsJson) as unknown : [];
    const stats = Array.isArray(statsValue) ? statsValue.filter(isRecord) : [];
    if (stats.length === 0) throw new HttpError(422, "MISSING_POSTGAME_STATS", "ROFL에서 경기 종료 플레이어 통계를 찾지 못했습니다.");

    return {
      fileName: basename(path),
      fileSize: stat.size,
      sha256: await hashFile(path, signal),
      gameVersion,
      patch,
      metadata: parsed,
      stats,
    };
  } catch (error) {
    if (error instanceof HttpError) throw error;
    if (error instanceof SyntaxError) throw new HttpError(400, "INVALID_ROFL", "ROFL 메타데이터 JSON이 손상되었습니다.", { cause: error });
    throw error;
  } finally {
    await handle.close();
  }
}
