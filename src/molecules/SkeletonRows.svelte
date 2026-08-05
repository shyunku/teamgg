<script>
  import Skeleton from "./Skeleton.svelte";

  export let rows = 5;
  export let height = "40px";
  export let gap = "6px";
</script>

<div class="skeleton-rows" style={`gap: ${gap}`} aria-label="불러오는 중" aria-busy="true">
  {#each Array(rows) as _}
    <Skeleton width="100%" {height} />
  {/each}
</div>

<style>
  .skeleton-rows {
    display: flex;
    width: 100%;
    flex-direction: column;
  }
</style>
