import type { AnalysisOptions, ReplayDigest, TextDeltaReporter } from "../domain.js";

export interface ReplayAnalyzer {
  readonly model: string;
  analyze(digest: ReplayDigest, options: AnalysisOptions, signal: AbortSignal): Promise<string>;
  analyzeStream(
    digest: ReplayDigest,
    options: AnalysisOptions,
    signal: AbortSignal,
    onDelta: TextDeltaReporter,
  ): Promise<string>;
}
