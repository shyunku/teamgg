<script>
  import Skeleton from "./Skeleton.svelte";
  import SkeletonRows from "./SkeletonRows.svelte";

  export let sections = 2;
  export let rows = 4;
  export let showHeading = true;
</script>

<div class="page-skeleton" aria-label="페이지를 불러오는 중" aria-busy="true">
  {#if showHeading}
    <div class="heading">
      <Skeleton width="34%" height="30px" />
      <Skeleton width="52%" height="13px" />
    </div>
  {/if}
  {#each Array(sections) as _}
    <div class="section card">
      <Skeleton width="26%" height="18px" />
      <SkeletonRows {rows} height="42px" gap="7px" />
    </div>
  {/each}
</div>

<style>
  .page-skeleton {
    display: flex;
    width: 100%;
    box-sizing: border-box;
    flex-direction: column;
    gap: 14px;
    padding: 24px 0;
  }

  .heading,
  .section {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .section {
    padding: 18px;
  }
</style>
