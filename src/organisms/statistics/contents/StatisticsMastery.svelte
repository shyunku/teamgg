<script>
  import moment from "moment";
  import SafeImg from "../../../atoms/SafeImg.svelte";
  import { championIconUrl, getMasteryStatisticsReq, profileIconUrl } from "../../../thunks/GeneralThunk";
  import JsxUtil from "../../../utils/JsxUtil";
  import { formatStd, formatStdKr } from "../../../utils/Util";
  import "./StatisticsMastery.scss";
  import { toasts } from "svelte-toasts";
  import Skeleton from "../../../molecules/Skeleton.svelte";
  import SkeletonRows from "../../../molecules/SkeletonRows.svelte";

  let rawData = null;

  let refinedData = {};
  let lastUpdateTime = null;

  let selectedChampionId = null;
  let rankers = [];

  const getMasteryStatistics = async () => {
    try {
      const resp = await getMasteryStatisticsReq();
      const { updatedAt, masteryGroups } = resp;

      rawData = masteryGroups;
      lastUpdateTime = updatedAt;

      refinedData = rawData.reduce(
        (acc, cur) => ({
          ...acc,
          [cur.championId]: cur,
        }),
        {}
      );
      // console.log(masteryGroups, refinedData);
    } catch (e) {
      console.error(e);
      rawData = [];
      toasts.add({
        title: "티어/랭크 통계",
        description: "통계를 불러오는 중 오류가 발생했습니다.",
        type: "error",
      });
    }
  };

  const goToPlayerPage = (gameName, tagLine) => {
    window.location.href = `#/player/${gameName}/${tagLine}`;
  };

  $: {
    if (selectedChampionId == null) {
      selectedChampionId = Object.values(refinedData)?.sort((a, b) => a.championName.localeCompare(b.championName))?.[0]
        ?.championId;
    }
    rankers = refinedData[selectedChampionId]?.rankers ?? [];
  }

  getMasteryStatistics();
</script>

<div class="statistics-mastery">
  <div class="content card">
    <div class="title">챔피언 숙련도 랭킹</div>
    <div class="description">해당 지표들은 team.gg에서 검색 또는 추적되는 플레이어들만 해당됩니다.</div>
    <div class="updated-at">
      {#if rawData == null}<Skeleton width="230px" height="12px" />{:else}{moment(lastUpdateTime).format("YYYY년 M월 D일 a h시 mm분에 업데이트됨")}{/if}
    </div>
  </div>
  <div class="mastery-content">
    <div class="champion-list card">
      <div class="header">챔피언별 통계 ({Object.keys(refinedData).length}명)</div>
      <div class="champions">
        <div class="champion header">
          <div class="champion-name">챔피언 이름</div>
          <div class="summoners">숙련도 7 이상</div>
          <div class="avg-mastery">평균 숙련도</div>
          <div class="max-mastery">최대 숙련도</div>
          <!-- <div class="total-mastery">숙련도 총합</div> -->
        </div>
        {#if rawData == null}
          <SkeletonRows rows={9} height="40px" gap="2px" />
        {:else}
        {#each Object.values(refinedData).sort((a, b) => a.championName.localeCompare(b.championName)) as mastery, i}
          <div
            class={"champion" + JsxUtil.classByEqual(selectedChampionId, mastery?.championId, "selected")}
            on:click={(e) => {
              selectedChampionId = mastery?.championId;
            }}
          >
            <div class="champion-icon img">
              <SafeImg src={championIconUrl(mastery?.championId)} loading="lazy" decoding="async" />
            </div>
            <div class="champion-name">{mastery?.championName}</div>
            <div class="summoners">{mastery?.masteredCount}명</div>
            <div class="avg-mastery">{formatStd(mastery?.avgMastery ?? 0)}점</div>
            <div class="max-mastery">{formatStd(mastery?.maxMastery)}점</div>
            <!-- <div class="total-mastery">{mastery?.totalMastery}</div> -->
          </div>
        {/each}
        {/if}
      </div>
    </div>
    <div class="summoner-list card">
      <div class="header">플레이어 순위 (상위 {(rankers ?? []).length}명)</div>
      <div class="summoners">
        {#if rawData == null}
          <SkeletonRows rows={9} height="40px" gap="2px" />
        {:else}
        {#each rankers ?? [] as r}
          <div class="summoner">
            <div class="summoner-rank">{r.ranks}위</div>
            <div class="profile-img img">
              <SafeImg src={profileIconUrl(r?.profileIconId)} loading="lazy" decoding="async" />
            </div>
            <div class="name-tag" on:click={(e) => goToPlayerPage(r.gameName, r.tagLine)}>
              <div class="game-name">{r.gameName}</div>
              <div class="tag-line">#{r.tagLine}</div>
            </div>
            <div class="league-points">{formatStd(r.championPoints)} 점</div>
          </div>
        {/each}
        {/if}
      </div>
    </div>
  </div>
</div>
