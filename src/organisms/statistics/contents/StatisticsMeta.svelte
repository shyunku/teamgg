<script>
  import SafeImg from "../../../atoms/SafeImg.svelte";
  import SortVisualizer from "../../../molecules/SortVisualizer.svelte";
  import {
    championIconUrl,
    getChampionStatisticsReq,
    getMetaStatisticsReq,
    itemIconUrl,
  } from "../../../thunks/GeneralThunk";
  import "./StatisticsChampion.scss";
  import { push } from "svelte-spa-router";
  import moment from "moment";
  import "moment/locale/ko";
  import { toasts } from "svelte-toasts";
  import LinePosition from "../../../molecules/LinePosition.svelte";
  import { MetaCategories } from "../../../types/General";
  import { compareVersions } from "compare-versions";
  import Skeleton from "../../../molecules/Skeleton.svelte";
  import SkeletonRows from "../../../molecules/SkeletonRows.svelte";
  moment.locale("ko");

  let rawData = null;
  let refinedData = [];
  let dataPatches = [];
  let lastUpdateTime = null;
  let rateMaxima = {
    winRate: 0,
    avgPickRate: 0,
    avgBanRate: 0,
  };
  let selectedLane = null;
  let searchQuery = "";

  let reverseSort = false;
  let sortBy = null;

  const laneOptions = [
    { key: "top", label: "탑" },
    { key: "jungle", label: "정글" },
    { key: "mid", label: "미드" },
    { key: "adc", label: "원거리 딜러" },
    { key: "support", label: "서포터" },
  ];

  const toggleLane = (lane) => {
    selectedLane = selectedLane === lane ? null : lane;
  };

  const applySortOption = (key) => {
    if (sortBy === key) {
      if (reverseSort) {
        reverseSort = false;
        sortBy = null;
        return;
      } else {
        reverseSort = true;
      }
    } else {
      reverseSort = false;
      sortBy = key;
    }
  };

  const sortOptions = [
    { key: "lane", label: "라인", class: "champion-position" },
    { key: "championName", label: "챔피언 이름", class: "champion-name" },
    { key: "metaItems", label: "메타", class: "champion-items", except: true },
    { key: "metaName", label: "분류", class: "champion-meta" },
    { key: "count", label: "플레이 수", class: "champion-played" },
    { key: "winRate", label: "평균 승률", class: "champion-winrate" },
    { key: "avgPickRate", label: "평균 픽률", class: "champion-pickrate" },
    { key: "avgBanRate", label: "평균 밴률", class: "champion-banrate" },
  ];

  const getChampionStatistics = async () => {
    try {
      const resp = await getMetaStatisticsReq();
      const { updatedAt, data, patches } = resp;
      lastUpdateTime = updatedAt;
      dataPatches = [...patches].sort((a, b) => compareVersions(b, a, ">="));
      rawData = [];
      if (Array.isArray(data)) {
        rawData = data.map((meta) => {
          let metaName = MetaCategories[meta.majorTag] ?? meta.majorTag;
          if (meta.minorTag != null) {
            metaName += ` / ${MetaCategories[meta.minorTag] ?? meta.minorTag}`;
          }
          return { ...meta, metaName };
        });
      } else {
        // Keep compatibility with servers deployed before the lightweight
        // meta summary response.
        for (let key in data) {
        const championData = data[key];
        const lanePickCount = Object.values(championData.metaTree).reduce((acc, cur) => acc + cur.pickCount, 0);
        for (let lane in championData.metaTree) {
          const laneData = championData.metaTree[lane] ?? null;
          const metaList = laneData?.metaPicks ?? [];
          const lanePickRate = laneData?.pickCount / lanePickCount;
          if (lanePickRate < 0.15) continue; // 15% 미만의 픽률 라인은 제외

          let maxWinRateMeta = null;
          let maxWinRate = 0;
          let maxPickRateMeta = null;
          let maxPickCount = 0;
          for (let meta of metaList) {
            let metaPickRate = meta.count / laneData.pickCount;
            // DataExplorer가 수집 중인 작은 표본에서도 메타가 전부
            // 사라지지 않도록 비율과 최소 판수를 함께 적용한다.
            if (metaPickRate < 0.1) continue; // 라인 내 10% 미만의 조합은 제외
            if (meta.count < 5) continue; // 표본이 지나치게 작은 조합은 제외
            if (meta.winRate > maxWinRate) {
              maxWinRate = meta.winRate;
              maxWinRateMeta = meta;
            }
            if (meta.count > maxPickCount) {
              maxPickCount = meta.count;
              maxPickRateMeta = meta;
            }
          }
          const collected = [maxWinRateMeta, maxPickRateMeta].filter((c) => c != null);
          if (collected.length == 0) continue;
          if (collected.length == 2) {
            if (maxWinRateMeta.metaKey == maxPickRateMeta.metaKey) {
              collected.pop();
            }
          }
          for (let meta of collected) {
            let metaName = MetaCategories[meta.majorTag] ?? meta.majorTag;
            if (meta.minorTag != null) {
              metaName += ` / ${MetaCategories[meta.minorTag] ?? meta.minorTag}`;
            }
            rawData.push({
              ...meta,
              lane: lane,
              metaName,
              championId: championData.championId,
              championName: championData.championName,
              avgPickRate: meta.count / laneData.pickCount,
              avgBanRate: championData.avgBanRate,
            });
          }
        }
      }
      }
      rawData = rawData.sort((a, b) => a.championName.localeCompare(b.championName));
    } catch (e) {
      console.error(e);
      rawData = [];
      toasts.add({
        title: "챔피언 메타",
        description: "메타를 불러오는 중 오류가 발생했습니다.",
        type: "error",
      });
    }
  };

  const moveToChampionDetail = (championId) => {
    push(`/statistics/champion/${championId}`);
  };

  const relativeBarWidth = (value, maximum) => {
    if (!Number.isFinite(value) || !Number.isFinite(maximum) || maximum <= 0) return 0;
    return Math.min(100, Math.max(0, (value / maximum) * 100));
  };

  $: if (rawData) {
    const normalizedQuery = searchQuery.trim().toLocaleLowerCase();
    refinedData = rawData
      .filter((c) => selectedLane == null || c?.lane === selectedLane)
      .filter((c) => !normalizedQuery || (c?.championName ?? "").toLocaleLowerCase().includes(normalizedQuery))
      .map((c) => {
        let extra = c?.extraStats ?? {};
        return {
          ...c,
          avgMinionsKilled: extra?.avgMinionsKilled ?? 0,
          avgGoldEarned: extra?.avgGoldEarned ?? 0,
        };
      })
      .sort((a, b) => {
        if (sortBy == null) return 0;
        if (typeof a[sortBy] === "string") {
          if (reverseSort) {
            return a[sortBy].localeCompare(b[sortBy]);
          } else {
            return b[sortBy].localeCompare(a[sortBy]);
          }
        }

        if (reverseSort) {
          return a[sortBy] - b[sortBy];
        } else {
          return b[sortBy] - a[sortBy];
        }
      });
  }

  $: rateMaxima = {
    winRate: Math.max(0, ...refinedData.map((c) => c.winRate ?? 0)),
    avgPickRate: Math.max(0, ...refinedData.map((c) => c.avgPickRate ?? 0)),
    avgBanRate: Math.max(0, ...refinedData.map((c) => c.avgBanRate ?? 0)),
  };

  getChampionStatistics();
</script>

<div class="statistics-champion">
  <div class="content card">
    <div class="statistics-heading-row">
      <div class="statistics-heading">
        <div class="title">챔피언 메타 통계 ({dataPatches.join(", ")} 패치)</div>
        <div class="description">해당 지표들은 team.gg에서 검색 또는 추적되는 데이터들로 구성되었습니다.</div>
        <div class="updated-at">
          {#if rawData == null}<Skeleton width="230px" height="12px" />{:else}{moment(lastUpdateTime).format("YYYY년 M월 D일 a h시 mm분에 업데이트됨")}{/if}
        </div>
      </div>
      <div class="statistics-filters">
        <div class="line-switches" aria-label="라인 필터">
          {#each laneOptions as lane}
            <button
              type="button"
              class:selected={selectedLane === lane.key}
              class="line-switch"
              title={lane.label}
              aria-label={`${lane.label} 필터`}
              aria-pressed={selectedLane === lane.key}
              disabled={rawData == null}
              on:click={() => toggleLane(lane.key)}
            >
              <LinePosition
                position={lane.key}
                size={17}
                opacity={1}
                highlightColor={selectedLane === lane.key ? "var(--color-highlight)" : null}
              />
            </button>
          {/each}
        </div>
        <input
          class="champion-search"
          type="search"
          bind:value={searchQuery}
          disabled={rawData == null}
          placeholder="챔피언 이름 검색"
          aria-label="챔피언 이름 검색"
        />
      </div>
    </div>
    <div class="champion-list">
      <div class="champion-item header">
        {#each sortOptions as option}
          <div class={option.class} on:mouseup={(e) => option?.except !== true && applySortOption(option.key)}>
            <div class="label">{option.label}</div>
            {#if option.except !== true}
              <div class="sort">
                <SortVisualizer direction={sortBy == option.key ? (reverseSort ? 1 : -1) : 0} />
              </div>
            {/if}
          </div>
        {/each}
      </div>
      {#if rawData == null}
        <div class="statistics-loading"><SkeletonRows rows={8} height="44px" gap="1px" /></div>
      {:else}
      {#each refinedData as c}
        {@const extra = c?.extraStats ?? {}}
        <div class="champion-item">
          <div class="champion-position">
            <LinePosition position={c?.lane} opacity={1} />
          </div>
          <div class="champion-img img">
            <SafeImg src={championIconUrl(c?.championId)} loading="lazy" decoding="async" />
          </div>
          <div class="champion-name" on:click={(e) => moveToChampionDetail(c?.championId)}>
            {c?.championName ?? "-"}
          </div>
          <div class="champion-items">
            {#each (c?.itemTree ?? []).slice(0, 3) as item}
              <div class="item img">
                <SafeImg src={itemIconUrl(item)} loading="lazy" decoding="async" />
              </div>
            {/each}
          </div>
          <div class="champion-meta">{c?.metaName ?? "-"}</div>
          <div class="champion-played">{c?.count ?? 0}판</div>
          <div class="champion-winrate">
            <div class="label">{(c.winRate * 100).toFixed(2)}%</div>
            <div class="bar-wrapper">
              <div class="bar" style={`width: ${relativeBarWidth(c.winRate, rateMaxima.winRate)}%`}></div>
            </div>
          </div>
          <div class="champion-pickrate">
            <div class="label">{(c.avgPickRate * 100).toFixed(2)}%</div>
            <div class="bar-wrapper">
              <div class="bar" style={`width: ${relativeBarWidth(c.avgPickRate, rateMaxima.avgPickRate)}%`}></div>
            </div>
          </div>
          <div class="champion-banrate">
            <div class="label">{(c.avgBanRate * 100).toFixed(2)}%</div>
            <div class="bar-wrapper">
              <div class="bar" style={`width: ${relativeBarWidth(c.avgBanRate, rateMaxima.avgBanRate)}%`}></div>
            </div>
          </div>
        </div>
      {/each}
      {#if rawData && refinedData.length === 0}
        <div class="empty-state">선택한 조건에 해당하는 메타가 없습니다.</div>
      {/if}
      {/if}
    </div>
  </div>
</div>
