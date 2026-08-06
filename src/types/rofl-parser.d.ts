declare module "rofl-parser" {
  export interface ParserProgress {
    stage: string;
    [key: string]: unknown;
  }

  export interface DecodeDetails {
    path: string;
    patch: string;
    gameVersion: string;
    artifactPath: string;
    artifactReused: boolean;
    summary: Record<string, unknown>;
    counts: Record<string, unknown>;
    outputFormat: "compact" | "verbose";
    executable?: Record<string, unknown>;
  }

  export interface DecodeOptions {
    replayPath: string;
    outputDirectory?: string;
    cacheDirectory?: string;
    executablePath?: string;
    outputFormat?: "compact" | "verbose";
    onProgress?: (progress: ParserProgress) => void;
    downloadOptions?: {
      region?: string;
      signal?: AbortSignal;
      [key: string]: unknown;
    };
  }

  export function decodeDetailed(options: DecodeOptions): Promise<DecodeDetails>;
  export function decodeAutoDetailed(options: DecodeOptions): Promise<DecodeDetails>;
}
