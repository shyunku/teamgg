<script>
  import { push } from "svelte-spa-router";
  import IoMdArrowBack from "svelte-icons/io/IoMdArrowBack.svelte";
  import IoIosCloudUpload from "svelte-icons/io/IoIosCloudUpload.svelte";
  import IoIosTrash from "svelte-icons/io/IoIosTrash.svelte";
  import MainContentLayout from "../layouts/MainContentLayout.svelte";
  import MarkdownRenderer from "../organisms/replay-analysis/MarkdownRenderer.svelte";
  import { deleteReplayAnalysis, openReplayAnalysisModal, replayAnalysisStore } from "../stores/ReplayAnalysisStore";
  import "./ReplayAnalysis.scss";

  export let params = {};

  const formatDate = (value) =>
    value ? new Intl.DateTimeFormat("ko-KR", { year: "numeric", month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(new Date(value)) : "";

  const goBack = () => {
    if (window.history.length > 1) window.history.back();
    else push("/custom-game");
  };
  const selectAnalysis = (id) => push(`/replay-analysis/${encodeURIComponent(id)}`);
  const removeAnalysis = (event, id) => {
    event.stopPropagation();
    if (!window.confirm("이 리플레이 분석 내역을 삭제하시겠습니까?")) return;
    const deletingCurrent = id === params.id;
    deleteReplayAnalysis(id);
    if (deletingCurrent) {
      const next = $replayAnalysisStore.history.find((entry) => entry.id !== id);
      if (next) selectAnalysis(next.id);
      else goBack();
    }
  };
  $: analysis = $replayAnalysisStore.history.find((entry) => entry.id === params.id) ?? null;
</script>

<svelte:head>
  <title>{analysis ? `${analysis.fileName} 분석` : "리플레이 분석"} | team.gg</title>
</svelte:head>

<MainContentLayout>
  <div class="replay-result-page">
    <aside class="analysis-sidebar card">
      <div class="sidebar-header">
        <strong>리플레이 분석</strong>
        <button title="새 리플레이 분석" on:click={() => openReplayAnalysisModal()}><IoIosCloudUpload /></button>
      </div>
      <div class="analysis-list">
        {#if $replayAnalysisStore.history.length === 0}
          <div class="empty">분석 내역이 없습니다.</div>
        {:else}
          {#each $replayAnalysisStore.history as item (item.id)}
            <div class:active={item.id === params.id} class="analysis-item" role="button" tabindex="0" on:click={() => selectAnalysis(item.id)} on:keydown={(event) => event.key === "Enter" && selectAnalysis(item.id)}>
              <div class="item-text"><strong>{item.fileName}</strong><span>{formatDate(item.createdAt)}</span></div>
              <button class="delete" title="삭제" aria-label="분석 내역 삭제" on:click={(event) => removeAnalysis(event, item.id)}><IoIosTrash /></button>
            </div>
          {/each}
        {/if}
      </div>
    </aside>

    <main class="analysis-content">
      <button class="back" on:click={goBack}><IoMdArrowBack /><span>뒤로가기</span></button>
      {#if analysis}
        <header class="result-header"><div><h1>{analysis.fileName}</h1><p>{formatDate(analysis.createdAt)} · {analysis.model ?? "AI 분석"}</p></div></header>
        <section class="result-card"><MarkdownRenderer source={analysis.analysis} /></section>
      {:else}
        <section class="missing-result">
          <h1>분석 결과를 찾을 수 없습니다.</h1>
          <p>삭제되었거나 이 브라우저에 저장되지 않은 분석입니다.</p>
          <button on:click={() => openReplayAnalysisModal()}>리플레이 분석하기</button>
        </section>
      {/if}
    </main>
  </div>
</MainContentLayout>
