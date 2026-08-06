import { decodeAutoDetailed, decodeDetailed, type DecodeDetails, type ParserProgress } from "rofl-parser";
import type { AppConfig } from "../config.js";
import type { ProgressReporter } from "../domain.js";
import { HttpError, errorMessage, throwIfAborted } from "../errors.js";

function progressValue(progress: ParserProgress): number | undefined {
  const completed = Number(progress.completedBytes ?? progress.completed ?? progress.current);
  const total = Number(progress.totalBytes ?? progress.total);
  return Number.isFinite(completed) && Number.isFinite(total) && total > 0
    ? Math.max(0, Math.min(1, completed / total))
    : undefined;
}

export async function decodeReplay(
  replayPath: string,
  outputDirectory: string,
  config: AppConfig,
  signal: AbortSignal,
  report: ProgressReporter,
): Promise<DecodeDetails> {
  const onProgress = (progress: ParserProgress) => {
    throwIfAborted(signal);
    const stage = String(progress.stage);
    const mapping = stage.startsWith("executable-")
      ? { stage: "executable-resolution" as const, message: "리플레이 버전에 맞는 LoL 실행 파일을 준비하고 있습니다." }
      : stage.startsWith("artifact-")
        ? { stage: "decoder-preparation" as const, message: "패킷 디코더를 준비하고 있습니다." }
        : { stage: "replay-decoding" as const, message: "ROFL 패킷을 해석하고 있습니다." };
    const numericProgress = progressValue(progress);
    report({ ...mapping, ...(numericProgress === undefined ? {} : { progress: numericProgress }), detail: { parserStage: stage } });
  };

  const common = {
    replayPath,
    outputDirectory,
    cacheDirectory: config.roflCacheDirectory,
    outputFormat: "compact" as const,
    onProgress,
  };
  try {
    if (config.executablePath) return await decodeDetailed({ ...common, executablePath: config.executablePath });
    return await decodeAutoDetailed({
      ...common,
      downloadOptions: { region: config.downloadRegion, signal },
    });
  } catch (error) {
    throwIfAborted(signal);
    throw new HttpError(422, "ROFL_DECODE_FAILED", `ROFL 해석에 실패했습니다: ${errorMessage(error)}`, { cause: error });
  }
}
