import { get, writable } from "svelte/store";
import {
  createCustomGameReplayAnalysisReq,
  deleteCustomGameReplayAnalysisReq,
  getCustomGameReplayAnalysesReq,
  getCustomGameReplayAnalysisReq,
} from "../thunks/GeneralThunk";

const RUNNING_STATUSES = ["queued", "uploading", "analyzing"];
const POLL_INTERVAL_MS = 2_000;

const initialState = {
  modalOpen: false,
  contextCustomGameId: null,
  contextCanManage: false,
  current: null,
  history: [],
  loadingHistory: false,
};

export const replayAnalysisStore = writable(initialState);

let pollTimer = null;
let pollCustomGameId = null;
let pollInFlight = false;

const errorMessage = (err) => {
  const data = err?.response?.data;
  return (
    data?.error?.message ??
    (typeof data?.error === "string" ? data.error : null) ??
    data?.message ??
    err?.message ??
    "리플레이 분석 중 알 수 없는 오류가 발생했습니다."
  );
};

const clampPercent = (value) => Math.max(0, Math.min(100, Math.round(value)));

const parseSseBlock = (block) => {
  let event = "message";
  const data = [];
  for (const line of block.split(/\r?\n/)) {
    if (line.startsWith(":")) continue;
    if (line.startsWith("event:")) event = line.slice(6).trim();
    if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
  }
  if (data.length === 0) return null;
  const rawData = data.join("\n");
  try {
    return { event, data: JSON.parse(rawData) };
  } catch (_) {
    return { event, data: rawData };
  }
};

const streamReplayAnalysis = (url, ticket, form, { onUploadProgress, onEvent }) =>
  new Promise((resolve, reject) => {
    const request = new XMLHttpRequest();
    let responseOffset = 0;
    let pending = "";
    let result = null;
    let streamError = null;

    const dispatchBlock = (block) => {
      const parsed = parseSseBlock(block);
      if (!parsed) return;
      onEvent(parsed.event, parsed.data);
      if (parsed.event === "result") result = parsed.data;
      if (parsed.event === "error") streamError = new Error(parsed.data?.message ?? "리플레이 분석에 실패했습니다.");
    };
    const consumeResponse = (final = false) => {
      const responseText = request.responseText ?? "";
      pending += responseText.slice(responseOffset);
      responseOffset = responseText.length;
      const blocks = pending.split(/\r?\n\r?\n/);
      pending = blocks.pop() ?? "";
      for (const block of blocks) dispatchBlock(block);
      if (final && pending.trim()) dispatchBlock(pending);
    };

    request.open("POST", url, true);
    request.timeout = 30 * 60 * 1000;
    request.setRequestHeader("Accept", "text/event-stream");
    request.setRequestHeader("X-Replay-Upload-Ticket", ticket);
    request.upload.onprogress = (event) => event.lengthComputable && onUploadProgress(event.loaded / event.total);
    request.onprogress = () => consumeResponse();
    request.onload = () => {
      consumeResponse(true);
      if (request.status < 200 || request.status >= 300) {
        try {
          const body = JSON.parse(request.responseText);
          reject(new Error(body?.error?.message ?? body?.message ?? `분석 서버가 ${request.status} 오류를 반환했습니다.`));
        } catch (_) {
          reject(new Error(`분석 서버가 ${request.status} 오류를 반환했습니다.`));
        }
      } else if (streamError) reject(streamError);
      else if (result) resolve(result);
      else reject(new Error("분석 서버가 완료 결과를 반환하지 않았습니다."));
    };
    request.onerror = () => reject(new Error("리플레이 분석 서버에 연결하지 못했습니다."));
    request.ontimeout = () => reject(new Error("리플레이 분석 제한 시간을 초과했습니다."));
    request.onabort = () => reject(new Error("리플레이 분석 요청이 취소되었습니다."));
    request.send(form);
  });

const syncPolling = (customGameId, analyses) => {
  const hasRunning = analyses.some((entry) => RUNNING_STATUSES.includes(entry.status));
  if (!hasRunning) {
    if (pollTimer && pollCustomGameId === customGameId) clearInterval(pollTimer);
    if (pollCustomGameId === customGameId) {
      pollTimer = null;
      pollCustomGameId = null;
    }
    return;
  }
  if (pollTimer && pollCustomGameId === customGameId) return;
  if (pollTimer) clearInterval(pollTimer);
  pollCustomGameId = customGameId;
  pollTimer = setInterval(async () => {
    if (pollInFlight) return;
    pollInFlight = true;
    try {
      await loadReplayAnalyses(customGameId, false);
    } catch (_) {
      // A temporary polling failure must not stop the shared job from running.
    } finally {
      pollInFlight = false;
    }
  }, POLL_INTERVAL_MS);
};

export const loadReplayAnalyses = async (customGameId, showLoading = true) => {
  if (!customGameId) return [];
  if (showLoading) replayAnalysisStore.update((state) => ({ ...state, loadingHistory: true }));
  try {
    const payload = await getCustomGameReplayAnalysesReq(customGameId);
    const analyses = Array.isArray(payload?.analyses) ? payload.analyses : [];
    const active = analyses.find((entry) => RUNNING_STATUSES.includes(entry.status)) ??
      analyses.find((entry) => entry.status === "failed") ?? null;
    replayAnalysisStore.update((state) => {
      if (state.contextCustomGameId && state.contextCustomGameId !== customGameId) return state;
      const persistedCurrent = analyses.find((entry) => entry.id === state.current?.id) ?? null;
      const currentSource = active ?? persistedCurrent;
      const current = currentSource
        ? {
            ...currentSource,
            uploadProgress: state.current?.id === currentSource.id ? state.current.uploadProgress : currentSource.status === "queued" ? 0 : 100,
            progress: Math.max(currentSource.progress ?? 0, state.current?.id === currentSource.id ? state.current.progress ?? 0 : 0),
            error: currentSource.error ?? null,
          }
        : null;
      return {
        ...state,
        contextCustomGameId: customGameId,
        contextCanManage: payload?.canManage === true,
        current,
        history: analyses.filter((entry) => entry.status === "completed"),
        loadingHistory: false,
      };
    });
    syncPolling(customGameId, analyses);
    return analyses;
  } catch (error) {
    replayAnalysisStore.update((state) => ({ ...state, loadingHistory: false }));
    throw error;
  }
};

export const loadReplayAnalysis = async (id) => {
  const payload = await getCustomGameReplayAnalysisReq(id);
  const analysis = payload?.analysis ?? null;
  if (!analysis) return null;
  replayAnalysisStore.update((state) => ({
    ...state,
    contextCustomGameId: analysis.customGameId,
    contextCanManage: payload?.canManage === true,
    history: [analysis, ...state.history.filter((entry) => entry.id !== analysis.id)],
  }));
  await loadReplayAnalyses(analysis.customGameId, false);
  replayAnalysisStore.update((state) => ({
    ...state,
    history: state.history.map((entry) => entry.id === analysis.id ? analysis : entry),
  }));
  return analysis;
};

export const openReplayAnalysisModal = ({ file = null, customGameId = null, canManage = false } = {}) => {
  replayAnalysisStore.update((state) => ({
    ...state,
    modalOpen: true,
    contextCustomGameId: customGameId ?? state.contextCustomGameId,
    contextCanManage: customGameId ? canManage : state.contextCanManage,
  }));
  const targetConfigId = customGameId ?? get(replayAnalysisStore).contextCustomGameId;
  if (targetConfigId) void loadReplayAnalyses(targetConfigId).catch(() => undefined);
  if (file) return startReplayAnalysis(file, targetConfigId, canManage);
  return Promise.resolve();
};

export const closeReplayAnalysisModal = () => {
  replayAnalysisStore.update((state) => ({ ...state, modalOpen: false }));
};

export const startReplayAnalysis = async (
  file,
  customGameId = get(replayAnalysisStore).contextCustomGameId,
  canManage = get(replayAnalysisStore).contextCanManage,
) => {
  if (!customGameId) throw new Error("내전 구성 정보가 없습니다.");
  if (!canManage) throw new Error("내전 리플레이 업로드는 방장만 할 수 있습니다.");
  const running = get(replayAnalysisStore).current;
  if (running && RUNNING_STATUSES.includes(running.status)) throw new Error("이미 다른 리플레이를 분석하고 있습니다.");
  if (!file || !String(file.name ?? "").toLowerCase().endsWith(".rofl")) throw new Error(".rofl 리플레이 파일만 업로드할 수 있습니다.");

  const created = await createCustomGameReplayAnalysisReq(customGameId, file.name, file.size);
  const job = created?.analysis;
  if (!job?.id || !created?.uploadUrl || !created?.uploadTicket) throw new Error("분석 작업 생성 응답이 올바르지 않습니다.");
  replayAnalysisStore.update((state) => ({
    ...state,
    current: { ...job, status: "uploading", progress: 0, uploadProgress: 0, stage: "리플레이 업로드 준비 중", error: null },
  }));

  const form = new FormData();
  form.append("replay", file, file.name);
  const separator = created.uploadUrl.includes("?") ? "&" : "?";
  const uploadUrl = `${created.uploadUrl}${separator}language=Korean`;
  try {
    const response = await streamReplayAnalysis(uploadUrl, created.uploadTicket, form, {
      onUploadProgress: (ratio) => replayAnalysisStore.update((state) => state.current?.id !== job.id ? state : ({
        ...state,
        current: {
          ...state.current,
          uploadProgress: clampPercent(ratio * 100),
          progress: clampPercent(ratio * 15),
          status: "uploading",
          stage: ratio >= 1 ? "업로드 완료 · 서버 응답 대기 중" : "리플레이 업로드 중",
        },
      })),
      onEvent: (event, payload) => replayAnalysisStore.update((state) => {
        if (state.current?.id !== job.id) return state;
        if (event === "started") return { ...state, current: { ...state.current, uploadProgress: 100, progress: Math.max(15, state.current.progress ?? 0), status: "analyzing", stage: "업로드 완료 · 분석 작업 준비 중" } };
        if (event === "result") return { ...state, current: { ...state.current, uploadProgress: 100, progress: 100, status: "completed", stage: "분석 완료" } };
        if (event !== "progress") return state;
        const serverProgress = Number(payload?.progress);
        const nextProgress = Number.isFinite(serverProgress) ? clampPercent(Math.max(0, Math.min(1, serverProgress)) * 100) : state.current.progress ?? 15;
        return { ...state, current: { ...state.current, uploadProgress: 100, progress: Math.max(state.current.progress ?? 15, nextProgress), status: "analyzing", stage: payload?.message ?? state.current.stage, analysisStage: payload?.stage ?? state.current.analysisStage } };
      }),
    });
    await loadReplayAnalyses(customGameId, false);
    return response;
  } catch (error) {
    await loadReplayAnalyses(customGameId, false).catch(() => undefined);
    const sharedCurrent = get(replayAnalysisStore).current;
    if (sharedCurrent?.id === job.id && RUNNING_STATUSES.includes(sharedCurrent.status)) throw error;
    replayAnalysisStore.update((state) => state.current?.id !== job.id ? state : ({ ...state, current: { ...state.current, status: "failed", stage: "분석 실패", error: errorMessage(error) } }));
    throw error;
  }
};

export const deleteReplayAnalysis = async (id) => {
  if (!get(replayAnalysisStore).contextCanManage) throw new Error("분석 내역은 방장만 삭제할 수 있습니다.");
  await deleteCustomGameReplayAnalysisReq(id);
  const customGameId = get(replayAnalysisStore).contextCustomGameId;
  if (customGameId) await loadReplayAnalyses(customGameId, false);
};

export const getReplayAnalysis = (id) => get(replayAnalysisStore).history.find((entry) => entry.id === id) ?? null;
