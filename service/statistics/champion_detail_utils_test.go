package statistics

import "testing"

func TestBuildMetaStatisticsKeepsOnlyListSummary(t *testing.T) {
	source := &ChampionDetailStatistics{
		Patches: []string{"16.15"},
		Data: map[int]ChampionDetailStatisticsItem{
			1: {
				ChampionId:   1,
				ChampionName: "Annie",
				AvgBanRate:   0.12,
				MetaTree: ChampionDetailStatisticsPositionMetaTree{
					Mid: &ChampionDetailStatisticsMetaTree{
						PickCount: 80,
						MetaPicks: []ChampionDetailStatisticsMeta{
							{MetaKey: "highest-win", MajorTag: "burst", ItemTree: []int{1, 2, 3, 4}, Count: 10, WinRate: 0.8},
							{MetaKey: "highest-pick", MajorTag: "mana", ItemTree: []int{5, 6, 7}, Count: 20, WinRate: 0.6},
							{MetaKey: "too-small", Count: 4, WinRate: 1},
						},
					},
					Support: &ChampionDetailStatisticsMetaTree{PickCount: 20},
				},
			},
		},
	}

	result := buildMetaStatistics(source)
	if result == nil || len(result.Data) != 2 {
		t.Fatalf("expected two summarized metas, got %#v", result)
	}
	if result.Data[0].MetaKey != "highest-win" || result.Data[1].MetaKey != "highest-pick" {
		t.Fatalf("unexpected summarized metas: %#v", result.Data)
	}
	if result.Data[0].Lane != "mid" {
		t.Fatalf("expected frontend lane key mid, got %s", result.Data[0].Lane)
	}
	if len(result.Data[0].ItemTree) != 3 {
		t.Fatalf("expected item tree to be trimmed to three items, got %v", result.Data[0].ItemTree)
	}
	if result.Data[0].AvgPickRate != 0.125 {
		t.Fatalf("expected lane-relative pick rate 0.125, got %f", result.Data[0].AvgPickRate)
	}
}
