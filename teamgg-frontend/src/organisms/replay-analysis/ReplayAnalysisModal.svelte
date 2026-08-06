<script>
  import { push } from "svelte-spa-router";
  import { toasts } from "svelte-toasts";
  import IoIosCloudUpload from "svelte-icons/io/IoIosCloudUpload.svelte";
  import IoIosTrash from "svelte-icons/io/IoIosTrash.svelte";
  import IoMdClose from "svelte-icons/io/IoMdClose.svelte";
  import {
    closeReplayAnalysisModal,
    deleteReplayAnalysis,
    replayAnalysisStore,
    startReplayAnalysis,
  } from "../../stores/ReplayAnalysisStore";
  import "./ReplayAnalysisModal.scss";

  let fileInput;
  let dragging = false;

  const formatFileSize = (size) => {
    if (!Number.isFinite(size)) return "";
    if (size < 1024 * 1024) return `${Math.max(1, Math.round(size / 1024))} KB`;
    return `${(size / 1024 / 1024).toFixed(1)} MB`;
  };

  const formatDate = (value) =>
    value
      ? new Intl.DateTimeFormat("ko-KR", {
          month: "numeric",
          day: "numeric",
          hour: "2-digit",
          minute: "2-digit",
        }).format(new Date(value))
      : "";

  const submitFile = async (file) => {
    if (!file) return;
    try {
      await startReplayAnalysis(file);
      toasts.add({
        title: "리플레이 분석 완료",
        description: "분석 결과가 저장되었습니다.",
        type: "success",
      });
    } catch (err) {
      toasts.add({
        title: "리플레이 분석 실패",
        description:
          err?.response?.data?.error?.message ??
          err?.message ??
          "리플레이를 분석하지 못했습니다.",
        type: "error",
      });
    } finally {
      if (fileInput) fileInput.value = "";
    }
  };

  const onDrop = (event) => {
    event.preventDefault();
    if (isRunning) return;
    dragging = false;
    const file = [...(event.dataTransfer?.files ?? [])].find((entry) =>
      entry.name.toLowerCase().endsWith(".rofl"),
    );
    if (!file) {
      toasts.add({
        title: "파일 형식 오류",
        description: ".rofl 리플레이 파일을 선택해주세요.",
        type: "warning",
      });
      return;
    }
    void submitFile(file);
  };

  const viewResult = (id) => {
    closeReplayAnalysisModal();
    push(`/replay-analysis/${encodeURIComponent(id)}`);
  };

  const removeResult = (event, id) => {
    event.stopPropagation();
    if (!window.confirm("이 리플레이 분석 내역을 삭제하시겠습니까?")) return;
    deleteReplayAnalysis(id);
  };

  $: isRunning = ["uploading", "analyzing"].includes($replayAnalysisStore.current?.status);
</script>

{#if $replayAnalysisStore.modalOpen}
  <div class="replay-modal-backdrop" role="presentation" on:click|self={closeReplayAnalysisModal}>
    <section class="replay-modal" role="dialog" aria-modal="true" aria-labelledby="replay-modal-title">
      <header>
        <div>
          <h2 id="replay-modal-title">내전 리플레이 분석</h2>
          <p>ROFL 파일을 업로드하면 경기 흐름과 승패 요인, 개인별 피드백을 AI가 분석합니다.</p>
        </div>
        <button class="close" aria-label="닫기" on:click={closeReplayAnalysisModal}><IoMdClose /></button>
      </header>

      <div class="modal-content">
        <div class="upload-panel">
          <div
            class:dragging
            class:disabled={isRunning}
            class="drop-zone"
            on:dragenter|preventDefault={() => (dragging = true)}
            on:dragover|preventDefault={() => (dragging = true)}
            on:dragleave|preventDefault={() => (dragging = false)}
            on:drop={onDrop}
            on:click={() => !isRunning && fileInput?.click()}
          >
            <div class="upload-icon"><IoIosCloudUpload /></div>
            <strong>{isRunning ? "리플레이를 처리하고 있습니다" : "ROFL 파일을 여기에 놓으세요"}</strong>
            <span>{isRunning ? "창을 닫아도 분석은 계속됩니다." : "클릭해서 파일을 직접 선택할 수도 있습니다."}</span>
            <input
              bind:this={fileInput}
              type="file"
              accept=".rofl"
              disabled={isRunning}
              on:change={(event) => void submitFile(event.currentTarget.files?.[0])}
            />
          </div>

          {#if $replayAnalysisStore.current}
            <div class:failed={$replayAnalysisStore.current.status === "error"} class="upload-progress-card">
              <div class="progress-header">
                <div class="file-info">
                  <strong>{$replayAnalysisStore.current.fileName}</strong>
                  <span>{formatFileSize($replayAnalysisStore.current.fileSize)}</span>
                </div>
                <span class="status">{$replayAnalysisStore.current.stage}</span>
              </div>
              <div class="progress-track">
                <div
                  class:analyzing={$replayAnalysisStore.current.status === "analyzing"}
                  class="progress-value"
                  style:width={`${$replayAnalysisStore.current.progress ?? 0}%`}
                ></div>
              </div>
              <div class="progress-footer">
                {#if $replayAnalysisStore.current.status === "error"}
                  <span class="error">{$replayAnalysisStore.current.error}</span>
                {:else if $replayAnalysisStore.current.status === "completed"}
                  <span>분석 결과가 준비되었습니다.</span>
                  <button on:click={() => viewResult($replayAnalysisStore.current.resultId)}>결과 보기</button>
                {:else}
                  <span>{$replayAnalysisStore.current.status === "analyzing" ? "AI 분석은 수 분 정도 걸릴 수 있습니다." : `${$replayAnalysisStore.current.progress ?? 0}% 업로드됨`}</span>
                {/if}
              </div>
            </div>
          {/if}

          <div class="upload-guide">
            <strong>리플레이 파일 찾는 방법</strong>
            <p>League of Legends 클라이언트에서 대전 기록을 다운로드한 뒤 리플레이 폴더의 <code>.rofl</code> 파일을 업로드하세요.</p>
            <p>분석을 위해 Riot ID와 경기 내 플레이 데이터가 분석 서버 및 AI 제공자에게 전송됩니다.</p>
          </div>
        </div>

        <aside class="history-panel">
          <div class="history-header">
            <strong>분석 내역</strong>
            <span>{$replayAnalysisStore.history.length}개</span>
          </div>
          <div class="history-list">
            {#if $replayAnalysisStore.history.length === 0}
              <div class="empty-history">아직 분석한 리플레이가 없습니다.</div>
            {:else}
              {#each $replayAnalysisStore.history as item (item.id)}
                <div class="history-item" role="button" tabindex="0" on:click={() => viewResult(item.id)} on:keydown={(event) => event.key === "Enter" && viewResult(item.id)}>
                  <div class="history-main">
                    <strong>{item.fileName}</strong>
                    <span>{formatDate(item.createdAt)} · {item.model ?? "AI 분석"}</span>
                  </div>
                  <button
                    class="delete"
                    aria-label="분석 내역 삭제"
                    title="삭제"
                    on:click={(event) => removeResult(event, item.id)}
                  ><IoIosTrash /></button>
                </div>
              {/each}
            {/if}
          </div>
        </aside>
      </div>
    </section>
  </div>
{/if}
