<script>
  import { onDestroy, onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import MainContentLayout from "../layouts/MainContentLayout.svelte";
  import Skeleton from "../molecules/Skeleton.svelte";
  import { getAdminEvents, getAdminOverview, getAdminSession } from "../thunks/AdminThunk";

  let loading = true;
  let refreshing = false;
  let denied = false;
  let error = "";
  let session = null;
  let overview = null;
  let eventData = { events: [], audits: [] };
  let timer;

  const formatNumber = (value) => Number(value ?? 0).toLocaleString("ko-KR");
  const formatBytes = (value) => {
    let size = Number(value ?? 0);
    const units = ["B", "KB", "MB", "GB", "TB"];
    let unit = 0;
    while (size >= 1024 && unit < units.length - 1) {
      size /= 1024;
      unit++;
    }
    return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
  };
  const formatDate = (value) => (value ? new Date(value).toLocaleString("ko-KR") : "-");
  const statusCount = (status) => overview?.operations?.replayAnalyses?.find((entry) => entry.status === status)?.count ?? 0;
  const queue = (kind, status) => overview?.operations?.dataExplorer?.diagnostics?.[kind]?.[status] ?? 0;

  const load = async (initial = false) => {
    if (initial) loading = true;
    else refreshing = true;
    error = "";
    try {
      session = await getAdminSession();
      [overview, eventData] = await Promise.all([getAdminOverview(), getAdminEvents(100)]);
      denied = false;
    } catch (caught) {
      const status = caught?.response?.status;
      denied = status === 401 || status === 403;
      error = denied ? "관리자 권한이 없습니다." : caught?.response?.data?.message ?? "관리자 정보를 불러오지 못했습니다.";
    } finally {
      loading = false;
      refreshing = false;
    }
  };

  onMount(() => {
    load(true);
    timer = setInterval(() => load(false), 30000);
  });
  onDestroy(() => clearInterval(timer));
</script>

<MainContentLayout>
  <main class="admin-page">
    <header class="admin-header">
      <div>
        <button class="back" on:click={() => push("/")}>← team.gg</button>
        <h1>운영 관리자</h1>
        <p>서비스 상태, DataExplorer, 통계 스냅샷 및 감사 이벤트</p>
      </div>
      {#if session}<span class="role">{session.userId} · {session.role}</span>{/if}
      <button class="refresh" disabled={refreshing || loading} on:click={() => load(false)}>{refreshing ? "갱신 중" : "새로고침"}</button>
    </header>

    {#if loading}
      <div class="loading-grid">{#each Array(6) as _}<Skeleton width="100%" height="130px" />{/each}</div>
    {:else if error}
      <section class="error-card" class:denied>
        <strong>{error}</strong>
        <span>{denied ? "권한이 있는 team.gg 계정으로 로그인해주세요." : "서버 연결과 관리자 환경변수를 확인해주세요."}</span>
      </section>
    {:else}
      <section class="section">
        <h2>서비스 상태 <small>{formatDate(overview.generatedAt)}</small></h2>
        <div class="status-grid">
          {#each overview.services ?? [] as service}
            <article class="status-card" class:healthy={service.healthy}>
              <span class="indicator"></span>
              <div><strong>{service.name}</strong><small>{service.statusCode || "연결 실패"} · {service.latencyMs}ms</small></div>
              <b>{service.healthy ? "정상" : "오류"}</b>
            </article>
          {/each}
        </div>
      </section>

      <section class="section">
        <h2>운영 개요</h2>
        <div class="metric-grid">
          <article><span>DB 사용량</span><strong>{formatBytes(overview.operations?.dataExplorer?.metrics?.DatabaseBytes)}</strong><small>회수 가능 {formatBytes(overview.operations?.dataExplorer?.metrics?.DatabaseFreeBytes)}</small></article>
          <article><span>소환사</span><strong>{formatNumber(overview.operations?.dataExplorer?.metrics?.SummonerRows)}</strong><small>오늘 +{formatNumber(overview.operations?.dataExplorer?.metrics?.DailySummonerRowGrowth)}</small></article>
          <article><span>경기</span><strong>{formatNumber(overview.operations?.dataExplorer?.metrics?.MatchRows)}</strong><small>오늘 +{formatNumber(overview.operations?.dataExplorer?.metrics?.DailyMatchRowGrowth)}</small></article>
          <article><span>숙련도</span><strong>{formatNumber(overview.operations?.dataExplorer?.metrics?.MasteryRows)}</strong><small>오늘 +{formatNumber(overview.operations?.dataExplorer?.metrics?.DailyMasteryRowGrowth)}</small></article>
          <article><span>소환사 큐</span><strong>{formatNumber(queue("SummonerJobs", "pending"))}</strong><small>처리 {formatNumber(queue("SummonerJobs", "processing"))} · 실패 {formatNumber(queue("SummonerJobs", "failed"))}</small></article>
          <article><span>경기 큐</span><strong>{formatNumber(queue("MatchJobs", "pending"))}</strong><small>처리 {formatNumber(queue("MatchJobs", "processing"))} · 실패 {formatNumber(queue("MatchJobs", "failed"))}</small></article>
          <article><span>리플레이 분석</span><strong>{formatNumber(statusCount("analyzing"))}</strong><small>완료 {formatNumber(statusCount("completed"))} · 실패 {formatNumber(statusCount("failed"))}</small></article>
          <article><span>DB 마이그레이션</span><strong>{overview.operations?.migration?.version ?? "-"}</strong><small>{overview.operations?.migration?.dirty ? "적용 실패/진행 중" : "정상"}</small></article>
        </div>
      </section>

      <section class="section split">
        <div>
          <h2>통계 스냅샷</h2>
          <div class="list">
            {#each overview.operations?.statistics ?? [] as snapshot}
              <div><strong>{snapshot.key}</strong><span>{formatDate(snapshot.updatedAt)}</span></div>
            {:else}<div class="empty">저장된 스냅샷이 없습니다.</div>{/each}
          </div>
        </div>
        <div>
          <h2>운영 이벤트</h2>
          <div class="list scroll">
            {#each eventData.events ?? [] as event}
              <div class="event"><span class:warning={event.level === "warning"} class:error={event.level === "error"}>{event.level}</span><strong>{event.message}</strong><small>{formatDate(event.createdAt)}</small></div>
            {:else}<div class="empty">최근 운영 이벤트가 없습니다.</div>{/each}
          </div>
        </div>
      </section>

      <section class="section">
        <h2>관리자 감사 로그</h2>
        <div class="audit-table">
          <div class="audit-head"><span>일시</span><span>사용자 UID</span><span>행위</span><span>결과</span></div>
          {#each eventData.audits ?? [] as audit}
            <div><span>{formatDate(audit.createdAt)}</span><span>{audit.actorUid}</span><span>{audit.action}</span><span class:success={audit.result === "success"}>{audit.result}</span></div>
          {:else}<div class="empty">감사 로그가 없습니다.</div>{/each}
        </div>
      </section>
    {/if}
  </main>
</MainContentLayout>

<style lang="scss">
  @import "../styles/variables.scss";
  .admin-page { width: 100%; padding: 36px 0 70px; color: $color-text-primary; }
  .admin-header { display: flex; align-items: center; gap: 14px; margin-bottom: 26px; h1 { margin: 5px 0 2px; font-size: 28px; } p { margin: 0; color: $color-text-muted; font-size: 13px; } > div { flex: 1; } }
  button { border: 1px solid $color-border; border-radius: 7px; color: $color-text-primary; background: $color-surface-raised; cursor: pointer; }
  .back { border: 0; padding: 0; color: $color-accent-light; background: transparent; }
  .refresh { padding: 9px 14px; &:disabled { opacity: .5; } }
  .role { padding: 7px 10px; border-radius: 7px; color: $color-accent-light; background: $color-accent-soft; font-size: 12px; }
  .section { margin-bottom: 18px; padding: 18px; border: 1px solid $color-border; border-radius: 10px; background: $color-surface; box-shadow: $shadow-card; h2 { margin: 0 0 14px; font-size: 15px; small { margin-left: 8px; color: $color-text-muted; font-weight: normal; } } }
  .status-grid, .metric-grid, .loading-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; }
  .loading-grid { grid-template-columns: repeat(3, 1fr); width: 100%; }
  .status-card { display: flex; align-items: center; gap: 10px; padding: 14px; border: 1px solid $color-border; border-radius: 8px; background: $color-surface-raised; .indicator { width: 9px; height: 9px; border-radius: 50%; background: $color-danger; box-shadow: 0 0 8px $color-danger; } div { display: flex; flex: 1; flex-direction: column; } small { color: $color-text-muted; } b { color: $color-danger; font-size: 12px; } &.healthy { .indicator { background: $color-success; box-shadow: 0 0 8px $color-success; } b { color: $color-success; } } }
  .metric-grid article { display: flex; min-height: 78px; flex-direction: column; padding: 13px; border: 1px solid $color-border; border-radius: 8px; background: $color-surface-raised; span, small { color: $color-text-muted; font-size: 12px; } strong { margin: 8px 0 3px; font-size: 18px; } }
  .split { display: grid; grid-template-columns: 1fr 1.4fr; gap: 18px; }
  .list { border: 1px solid $color-border; border-radius: 7px; overflow: hidden; > div { display: flex; gap: 10px; padding: 10px 12px; border-bottom: 1px solid $color-divider-subtle; font-size: 12px; &:last-child { border-bottom: 0; } strong { flex: 1; } span, small { color: $color-text-muted; } } &.scroll { max-height: 250px; overflow-y: auto; } }
  .event span { width: 50px; color: $color-info !important; &.warning { color: $color-warning !important; } &.error { color: $color-danger !important; } }
  .audit-table { overflow-x: auto; > div { display: grid; grid-template-columns: 170px 1.5fr 1.5fr 80px; min-width: 700px; padding: 9px 12px; border-bottom: 1px solid $color-divider-subtle; font-size: 12px; span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; } .success { color: $color-success; } } .audit-head { color: $color-text-muted; background: $color-surface-raised; } }
  .error-card { display: flex; flex-direction: column; gap: 8px; padding: 25px; border: 1px solid $color-danger; border-radius: 9px; color: $color-danger; background: $color-surface; span { color: $color-text-muted; } }
  .empty { color: $color-text-muted; }
  @media (max-width: 900px) { .status-grid, .metric-grid, .loading-grid { grid-template-columns: repeat(2, 1fr); } .split { grid-template-columns: 1fr; } }
  @media (max-width: 560px) { .admin-header { align-items: flex-start; flex-wrap: wrap; } .status-grid, .metric-grid, .loading-grid { grid-template-columns: 1fr; } }
</style>
