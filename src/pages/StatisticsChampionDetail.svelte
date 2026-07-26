<script>
  import ChampionDetailContent from "../organisms/player/champion-statistics-detail/ChampionDetailContent.svelte";
  import ChampionDetailHeader from "../organisms/player/champion-statistics-detail/ChampionDetailHeader.svelte";
  import ChampionDetailMenu from "../organisms/player/champion-statistics-detail/ChampionDetailMenu.svelte";
  import { getChampionDetailStatisticsReq, getChampionStatisticsReq } from "../thunks/GeneralThunk";
  import "./StatisticsChampionDetail.scss";

  export const ChampionDetailOptions = {
    META: { key: "meta", label: "빌드 및 메타" },
    SKILL: { key: "skill", label: "스킬 설명 (Beta)" },
  };

  export let params = {};

  let data = null;
  let rateMaxima = {
    pickRate: 0,
    winRate: 0,
    banRate: 0,
  };
  let menuKey = ChampionDetailOptions.META.key;

  $: {
    if (params?.championId != null) {
      console.log(">> load champion detail", params.championId);
      loadChampionDetail(params.championId);
    }
  }

  let loadChampionDetail = async (championId) => {
    try {
      const [detailResponse, statisticsResponse] = await Promise.all([
        getChampionDetailStatisticsReq(championId),
        getChampionStatisticsReq(),
      ]);
      const champions = Object.values(statisticsResponse?.data ?? {});
      rateMaxima = {
        pickRate: Math.max(0, ...champions.map((champion) => champion?.avgPickRate ?? 0)),
        winRate: Math.max(
          0,
          ...champions.map((champion) => champion?.avgWinRate ?? (champion?.win ?? 0) / (champion?.total || 1))
        ),
        banRate: Math.max(0, ...champions.map((champion) => champion?.avgBanRate ?? 0)),
      };
      console.log(detailResponse);
      data = detailResponse;
    } catch (e) {
      console.error(e);
    }
  };
</script>

<ChampionDetailHeader {data} {rateMaxima} />
<ChampionDetailMenu menus={ChampionDetailOptions} bind:menuKey />
<ChampionDetailContent {menuKey} {data} />
