<script>
  import { toasts } from "svelte-toasts";
  import CustomGameContent from "../organisms/custom-game-config/CustomGameContent.svelte";
  import CustomGameHeader from "../organisms/custom-game-config/CustomGameHeader.svelte";
  import CustomGameSummary from "../organisms/custom-game-config/CustomGameSummary.svelte";
  import { getCustomGameBalanceReq, getCustomGameConfigurationInfo } from "../thunks/GeneralThunk";
  import { onDestroy, onMount } from "svelte";
  import { socketStore } from "../stores/SocketStore";
  import { loadReplayAnalyses, openReplayAnalysisModal } from "../stores/ReplayAnalysisStore";
  import MainContentLayout from "../layouts/MainContentLayout.svelte";
  import PageSkeleton from "../molecules/PageSkeleton.svelte";

  export let params = {};
  let data = null;
  let dataIndex = 0;
  let team1TotalRatingPoint = 0;
  let team2TotalRatingPoint = 0;

  let candidates = [];
  let weights = null;

  let team1ParticipantsMap = {};
  let team2ParticipantsMap = {};

  let socketConnected = false;
  let canManage = false;
  let ownedPuuids = [];
  let isOptimizing = false;
  let viewingPuuids = [];
  let unsubscribeSocket = () => {};
  let replayDragDepth = 0;
  let replayFileDragging = false;
  let loading = true;

  const containsFiles = (event) => [...(event.dataTransfer?.types ?? [])].includes("Files");
  const onReplayDragEnter = (event) => {
    if (!containsFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
    replayDragDepth += 1;
    replayFileDragging = true;
  };
  const onReplayDragOver = (event) => {
    if (!containsFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = canManage ? "copy" : "none";
  };
  const onReplayDragLeave = (event) => {
    if (!replayFileDragging) return;
    event.stopPropagation();
    replayDragDepth = Math.max(0, replayDragDepth - 1);
    if (replayDragDepth === 0) replayFileDragging = false;
  };
  const onReplayDrop = (event) => {
    const files = [...(event.dataTransfer?.files ?? [])];
    if (files.length === 0) return;
    event.preventDefault();
    event.stopPropagation();
    replayDragDepth = 0;
    replayFileDragging = false;
    if (!canManage) {
      toasts.add({ title: "권한 없음", description: "내전 리플레이 업로드는 방장만 할 수 있습니다.", type: "warning" });
      return;
    }
    const replay = files.find((file) => file.name.toLowerCase().endsWith(".rofl"));
    if (!replay) {
      toasts.add({ title: "파일 형식 오류", description: ".rofl 리플레이 파일을 선택해주세요.", type: "warning" });
      return;
    }
    void openReplayAnalysisModal({ file: replay, customGameId: params.id, canManage }).catch((err) => {
      toasts.add({
        title: "리플레이 분석 실패",
        description: err?.response?.data?.error?.message ?? err?.message ?? "리플레이 분석을 시작하지 못했습니다.",
        type: "error",
      });
    });
  };

  const onViewersChanged = (payload) => {
    viewingPuuids = payload?.puuids ?? [];
  };

  const onReplayAnalysisUpdated = () => {
    void loadReplayAnalyses(params.id, false).catch(() => undefined);
  };

  const updateBalance = async () => {
    try {
      const resp = await getCustomGameBalanceReq(params.id);
      data.balance = resp;
      console.log(resp);
    } catch (err) {
      console.error(err);
      toasts.add({
        title: "밸런스 업데이트 실패",
        description: "밸런스 업데이트에 실패했습니다.",
        duration: 3000,
        type: "error",
      });
    }
  };

  const fetchAllData = async () => {
    const initialLoad = data == null;
    if (initialLoad) loading = true;
    try {
      const resp = await getCustomGameConfigurationInfo(params.id);
      data = resp;
      dataIndex++;

      console.log(data);

      candidates = data.candidates;
      team1ParticipantsMap = data.team1.reduce((acc, cur) => {
        acc[cur?.position] = cur?.puuid;
        return acc;
      }, {});
      team2ParticipantsMap = data.team2.reduce((acc, cur) => {
        acc[cur?.position] = cur?.puuid;
        return acc;
      }, {});
      weights = data.weights;
      canManage = data.canManage === true;
      ownedPuuids = data.ownedPuuids ?? [];
      isOptimizing = data.isOptimizing === true;
      console.log(resp);
    } catch (err) {
      console.error(err);
    } finally {
      if (initialLoad) loading = false;
    }
  };

  onMount(() => {
    fetchAllData();
    socketStore.initialize();
    socketStore.on("custom_config/viewers", onViewersChanged);
    socketStore.on("custom_config/replay_analysis_updated", onReplayAnalysisUpdated);
    unsubscribeSocket = socketStore.subscribe((value) => {
      socketConnected = value?.connected;
      if (value?.connected) {
        socketStore.emit("join_custom_config_room", params.id);
      }
    });
    socketStore.on("custom_config/updated", () => {
      fetchAllData();
    });
  });

  onDestroy(() => {
    socketStore.off("custom_config/viewers", onViewersChanged);
    socketStore.off("custom_config/replay_analysis_updated", onReplayAnalysisUpdated);
    unsubscribeSocket();
    socketStore.disconnect();
  });
</script>

<svelte:window
  on:dragenter|capture={onReplayDragEnter}
  on:dragover|capture={onReplayDragOver}
  on:dragleave|capture={onReplayDragLeave}
  on:drop|capture={onReplayDrop}
/>

{#if replayFileDragging}
  <div class="replay-page-drop-overlay">
    <div>
      <strong>내전 리플레이 분석</strong>
      <span>{canManage ? "ROFL 파일을 놓아 업로드하세요." : "리플레이 업로드는 방장만 할 수 있습니다."}</span>
    </div>
  </div>
{/if}
<svelte:head>
  <title>내전 팀 구성</title>
</svelte:head>

{#if loading}
  <MainContentLayout>
    <PageSkeleton sections={3} rows={4} />
  </MainContentLayout>
{:else}
<CustomGameHeader
  configId={data?.id}
  name={data?.name}
  lastUpdatedAt={data?.lastUpdatedAt}
  canEdit={canManage}
  locked={isOptimizing}
  onNameChanged={(name, lastUpdatedAt) => {
    data = { ...data, name, lastUpdatedAt };
  }}
/>
<CustomGameSummary
  balance={data?.balance}
  configId={data?.id}
  {dataIndex}
  {weights}
  {canManage}
  {isOptimizing}
  onOptimizingChanged={(value) => {
    isOptimizing = value;
    if (data) data = { ...data, isOptimizing: value };
  }}
  {socketConnected}
  {fetchAllData}
  bind:team1TotalRatingPoint
  bind:team2TotalRatingPoint
/>
<CustomGameContent
  bind:team1TotalRatingPoint
  bind:team2TotalRatingPoint
  configId={data?.id}
  {candidates}
  {team1ParticipantsMap}
  {team2ParticipantsMap}
  {updateBalance}
  {fetchAllData}
  {canManage}
  {ownedPuuids}
  {isOptimizing}
  {viewingPuuids}
/>
{/if}
<style lang="scss">
  @import "../styles/variables.scss";

  .replay-page-drop-overlay {
    position: fixed;
    z-index: 1100;
    inset: 58px 0 0;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
    background: rgba(2, 6, 20, 0.76);
    backdrop-filter: blur(5px);

    > div {
      display: flex;
      width: min(520px, calc(100vw - 40px));
      height: 180px;
      align-items: center;
      justify-content: center;
      flex-direction: column;
      border: 2px dashed $color-highlight;
      border-radius: 12px;
      background: rgba(79, 140, 255, 0.08);
      box-shadow: 0 0 32px $color-highlight-glow;
    }

    strong { color: $color-text-primary; font-size: 20px; }
    span { margin-top: 9px; color: $color-highlight-light; font-size: 13px; }
  }
</style>
