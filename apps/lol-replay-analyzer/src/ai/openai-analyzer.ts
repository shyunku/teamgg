import OpenAI from "openai";
import type { AppConfig } from "../config.js";
import type { AnalysisOptions, ReplayDigest, TextDeltaReporter } from "../domain.js";
import { HttpError } from "../errors.js";
import type { ReplayAnalyzer } from "./analyzer.js";
import { buildAnalysisInput, SYSTEM_INSTRUCTIONS } from "./prompt.js";

export class OpenAIReplayAnalyzer implements ReplayAnalyzer {
  readonly model: string;
  private readonly client: OpenAI;
  private readonly maxOutputTokens: number;
  private readonly reasoningEffort: AppConfig["openaiReasoningEffort"];

  constructor(config: AppConfig) {
    if (!config.openaiApiKey || !config.openaiModel) {
      throw new HttpError(503, "AI_NOT_CONFIGURED", "OPENAI_API_KEY와 OPENAI_MODEL 설정이 필요합니다.");
    }
    this.model = config.openaiModel;
    this.maxOutputTokens = config.openaiMaxOutputTokens;
    this.reasoningEffort = config.openaiReasoningEffort;
    this.client = new OpenAI({ apiKey: config.openaiApiKey, baseURL: config.openaiBaseUrl });
  }

  async analyze(digest: ReplayDigest, options: AnalysisOptions, signal: AbortSignal): Promise<string> {
    const response = await this.client.responses.create({
      model: this.model,
      instructions: SYSTEM_INSTRUCTIONS,
      input: buildAnalysisInput(digest, options),
      max_output_tokens: this.maxOutputTokens,
      ...(this.reasoningEffort ? { reasoning: { effort: this.reasoningEffort } } : {}),
      store: false,
    }, { signal });
    const output = response.output_text.trim();
    if (!output) throw new HttpError(502, "EMPTY_AI_RESPONSE", "AI가 빈 분석 결과를 반환했습니다.");
    return output;
  }

  async analyzeStream(
    digest: ReplayDigest,
    options: AnalysisOptions,
    signal: AbortSignal,
    onDelta: TextDeltaReporter,
  ): Promise<string> {
    const stream = await this.client.responses.create({
      model: this.model,
      instructions: SYSTEM_INSTRUCTIONS,
      input: buildAnalysisInput(digest, options),
      max_output_tokens: this.maxOutputTokens,
      ...(this.reasoningEffort ? { reasoning: { effort: this.reasoningEffort } } : {}),
      store: false,
      stream: true,
    }, { signal });
    let output = "";
    for await (const event of stream) {
      if (event.type === "response.output_text.delta") {
        output += event.delta;
        onDelta(event.delta);
      }
    }
    if (!output.trim()) throw new HttpError(502, "EMPTY_AI_RESPONSE", "AI가 빈 분석 결과를 반환했습니다.");
    return output.trim();
  }
}
