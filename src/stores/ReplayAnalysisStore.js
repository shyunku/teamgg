import axios from "axios";
import { get, writable } from "svelte/store";

const STORAGE_KEY = "teamgg-replay-analysis-history-v1";
const MAX_HISTORY = 30;

const normalizeHost = (host) => {
  const value = String(host ?? "").trim().replace(/\/+$/, "");
  if (!value) return "http://localhost:7720";
  if (/^https?:\/\//i.test(value)) return value;
  return `https://${value}`;
};

export const ReplayAnalyzerHost = normalizeHost(
  typeof APP_REPLAY_ANALYZER_HOST === "undefined" ? "http://localhost:7720" : APP_REPLAY_ANALYZER_HOST,
);

const loadHistory = () => {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]");
    return Array.isArray(parsed) ? parsed : [];
  } catch (_) {
    return [];
  }
};

const initialState = {
  modalOpen: false,
  contextCustomGameId: null,
  current: null,
  history: loadHistory(),
};

export const replayAnalysisStore = writable(initialState);

replayAnalysisStore.subscribe((state) => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state.history.slice(0, MAX_HISTORY)));
  } catch (err) {
    console.warn("Replay analysis history could not be persisted", err);
  }
});

const createId = () =>
  globalThis.crypto?.randomUUID?.() ?? `replay-${Date.now()}-${Math.random().toString(36).slice(2)}`;

const errorMessage = (err) =>
  err?.response?.data?.error?.message ??
  err?.message ??
  "리플레이 분석 중 알 수 없는 오류가 발생했습니다.";

export const openReplayAnalysisModal = ({ file = null, customGameId = null } = {}) => {
  replayAnalysisStore.update((state) => ({ ...state, modalOpen: true, contextCustomGameId: customGameId }));
  if (file) void startReplayAnalysis(file, customGameId).catch(() => undefined);
};

export const closeReplayAnalysisModal = () => {
  replayAnalysisStore.update((state) => ({ ...state, modalOpen: false }));
};

export const startReplayAnalysis = async (file, customGameId = get(replayAnalysisStore).contextCustomGameId) => {
  const running = get(replayAnalysisStore).current;
  if (running && ["uploading", "analyzing"].includes(running.status)) {
    throw new Error("이미 다른 리플레이를 분석하고 있습니다.");
  }
  if (!file || !String(file.name ?? "").toLowerCase().endsWith(".rofl")) {
    throw new Error(".rofl 리플레이 파일만 업로드할 수 있습니다.");
  }

  const id = createId();
  const startedAt = new Date().toISOString();
  replayAnalysisStore.update((state) => ({
    ...state,
    modalOpen: true,
    current: {
      id,
      fileName: file.name,
      fileSize: file.size,
      customGameId,
      status: "uploading",
      progress: 0,
      stage: "리플레이 업로드 준비 중",
      startedAt,
      error: null,
    },
  }));

  const form = new FormData();
  form.append("replay", file, file.name);

  try {
    const response = await axios.post(
      `${ReplayAnalyzerHost}/v1/replays/analyze?language=Korean`,
      form,
      {
        timeout: 30 * 60 * 1000,
        onUploadProgress: (event) => {
          const ratio = event.total ? event.loaded / event.total : 0;
          const progress = Math.max(1, Math.min(100, Math.round(ratio * 100)));
          replayAnalysisStore.update((state) => {
            if (state.current?.id !== id) return state;
            return {
              ...state,
              current: {
                ...state.current,
                progress,
                status: progress >= 100 ? "analyzing" : "uploading",
                stage: progress >= 100 ? "업로드 완료 · AI 분석 중" : "리플레이 업로드 중",
              },
            };
          });
        },
      },
    );

    const result = {
      id,
      requestId: response.data?.requestId ?? null,
      fileName: file.name,
      fileSize: file.size,
      customGameId,
      analysis: String(response.data?.analysis ?? ""),
      model: response.data?.model ?? null,
      createdAt: new Date().toISOString(),
    };
    if (!result.analysis.trim()) throw new Error("분석 서버가 빈 결과를 반환했습니다.");

    replayAnalysisStore.update((state) => ({
      ...state,
      current: {
        ...state.current,
        status: "completed",
        progress: 100,
        stage: "분석 완료",
        completedAt: result.createdAt,
        resultId: id,
      },
      history: [result, ...state.history.filter((entry) => entry.id !== id)].slice(0, MAX_HISTORY),
    }));
    return result;
  } catch (err) {
    replayAnalysisStore.update((state) => {
      if (state.current?.id !== id) return state;
      return {
        ...state,
        current: {
          ...state.current,
          status: "error",
          stage: "분석 실패",
          error: errorMessage(err),
        },
      };
    });
    throw err;
  }
};

export const deleteReplayAnalysis = (id) => {
  replayAnalysisStore.update((state) => ({
    ...state,
    current: state.current?.resultId === id ? null : state.current,
    history: state.history.filter((entry) => entry.id !== id),
  }));
};

export const getReplayAnalysis = (id) =>
  get(replayAnalysisStore).history.find((entry) => entry.id === id) ?? null;
