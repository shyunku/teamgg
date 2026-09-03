package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	defaultMasteryBenchmarkSummoners  = 20
	defaultMasteryBenchmarkChampions  = 10
	defaultMasteryBenchmarkIterations = 5
)

type MasteryReadBenchmarkOptions struct {
	SummonerSamples int
	ChampionSamples int
	Iterations      int
}

type masteryReadBenchmarkTiming struct {
	Samples int     `json:"samples"`
	P50Ms   float64 `json:"p50Ms"`
	P95Ms   float64 `json:"p95Ms"`
	MeanMs  float64 `json:"meanMs"`
}

type masteryReadBenchmarkPair struct {
	Legacy    masteryReadBenchmarkTiming `json:"legacy"`
	NumericV2 masteryReadBenchmarkTiming `json:"numericV2"`
	Speedup   float64                    `json:"speedup"`
}

type MasteryReadBenchmarkResult struct {
	SummonerSamples int                      `json:"summonerSamples"`
	ChampionSamples int                      `json:"championSamples"`
	Iterations      int                      `json:"iterations"`
	ResultsEqual    bool                     `json:"resultsEqual"`
	SummonerLookup  masteryReadBenchmarkPair `json:"summonerLookup"`
	Aggregate       masteryReadBenchmarkPair `json:"aggregate"`
	TopRankers      masteryReadBenchmarkPair `json:"topRankers"`
}

func (r MasteryReadBenchmarkResult) String() string {
	encoded, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf("marshal benchmark result: %v", err)
	}
	return string(encoded)
}

func MasteryReadBenchmarkOptionsFromEnvironment() MasteryReadBenchmarkOptions {
	return MasteryReadBenchmarkOptions{
		SummonerSamples: boundedMasteryBenchmarkOption(os.Getenv("MASTERY_BENCHMARK_SUMMONERS"), defaultMasteryBenchmarkSummoners, 1, 100),
		ChampionSamples: boundedMasteryBenchmarkOption(os.Getenv("MASTERY_BENCHMARK_CHAMPIONS"), defaultMasteryBenchmarkChampions, 1, 100),
		Iterations:      boundedMasteryBenchmarkOption(os.Getenv("MASTERY_BENCHMARK_ITERATIONS"), defaultMasteryBenchmarkIterations, 1, 20),
	}
}

func boundedMasteryBenchmarkOption(value string, fallback, minimum, maximum int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}

type masteryBenchmarkRow struct {
	ChampionID                   int64         `db:"champion_id"`
	ChampionPointsUntilNextLevel int64         `db:"champion_points_until_next_level"`
	ChestGranted                 bool          `db:"chest_granted"`
	LastPlayTime                 time.Time     `db:"last_play_time"`
	ChampionLevel                int           `db:"champion_level"`
	ChampionPoints               int           `db:"champion_points"`
	ChampionPointsSinceLastLevel int64         `db:"champion_points_since_last_level"`
	TokensEarned                 int           `db:"tokens_earned"`
	SummonerFK                   sql.NullInt64 `db:"summoner_fk"`
}

type masteryBenchmarkAggregate struct {
	Maximum       int64 `db:"maximum"`
	Total         int64 `db:"total"`
	MasteredCount int64 `db:"mastered_count"`
	Rows          int64 `db:"row_count"`
}

type masteryBenchmarkRanker struct {
	Puuid          sql.NullString `db:"puuid"`
	ChampionPoints int            `db:"champion_points"`
}

const legacyMasteryLookupQuery = `
	SELECT champion_id, champion_points_until_next_level, chest_granted,
		last_play_time, champion_level, champion_points,
		champion_points_since_last_level, tokens_earned, summoner_fk
	FROM masteries
	WHERE puuid = ?`

const numericMasteryLookupQuery = `
	SELECT mastery.champion_id, mastery.champion_points_until_next_level,
		mastery.chest_granted, mastery.last_play_time, mastery.champion_level,
		mastery.champion_points, mastery.champion_points_since_last_level,
		mastery.tokens_earned, mastery.summoner_fk
	FROM summoner_numeric_keys numeric_key
	INNER JOIN masteries_numeric_v2 mastery
		ON mastery.summoner_fk = numeric_key.summoner_id
	WHERE numeric_key.puuid = ?`

const legacyMasteryAggregateQuery = `
	SELECT COALESCE(MAX(champion_points), 0) AS maximum,
		COALESCE(SUM(champion_points), 0) AS total,
		COALESCE(SUM(IF(champion_level >= 7, 1, 0)), 0) AS mastered_count,
		COUNT(*) AS row_count
	FROM masteries FORCE INDEX (masteries_champion_points_level_covering_index)
	WHERE champion_id = ?`

const numericMasteryAggregateQuery = `
	SELECT COALESCE(MAX(champion_points), 0) AS maximum,
		COALESCE(SUM(champion_points), 0) AS total,
		COALESCE(SUM(IF(champion_level >= 7, 1, 0)), 0) AS mastered_count,
		COUNT(*) AS row_count
	FROM masteries_numeric_v2
		FORCE INDEX (masteries_numeric_champion_points_level_covering_index)
	WHERE champion_id = ?`

const legacyMasteryTopRankersQuery = `
	SELECT s.puuid, m.champion_points
	FROM masteries m FORCE INDEX (masteries_champion_points_level_covering_index)
	LEFT JOIN summoners s ON m.puuid = s.puuid
	WHERE m.champion_id = ?
	ORDER BY m.champion_points DESC
	LIMIT 30`

const numericMasteryTopRankersQuery = `
	SELECT s.puuid, m.champion_points
	FROM masteries_numeric_v2 m
		FORCE INDEX (masteries_numeric_champion_points_level_covering_index)
	INNER JOIN summoner_numeric_keys numeric_key
		ON m.summoner_fk = numeric_key.summoner_id
	LEFT JOIN summoners s ON numeric_key.puuid = s.puuid
	WHERE m.champion_id = ?
	ORDER BY m.champion_points DESC
	LIMIT 30`

func BenchmarkMasteryReads(ctx context.Context, database *sqlx.DB, options MasteryReadBenchmarkOptions) (MasteryReadBenchmarkResult, error) {
	result := MasteryReadBenchmarkResult{}
	legacyExists, err := tableExists(ctx, database, "masteries")
	if err != nil {
		return result, err
	}
	if !legacyExists {
		return result, errors.New("legacy masteries has been retired; comparative read benchmark is no longer available")
	}
	options.SummonerSamples = boundedMasteryBenchmarkOption(strconv.Itoa(options.SummonerSamples), defaultMasteryBenchmarkSummoners, 1, 100)
	options.ChampionSamples = boundedMasteryBenchmarkOption(strconv.Itoa(options.ChampionSamples), defaultMasteryBenchmarkChampions, 1, 100)
	options.Iterations = boundedMasteryBenchmarkOption(strconv.Itoa(options.Iterations), defaultMasteryBenchmarkIterations, 1, 20)
	if err := ValidateMasteryNumericShadowCutover(ctx, database); err != nil {
		return result, fmt.Errorf("validate numeric mastery reads before benchmark: %w", err)
	}

	var puuids []string
	if err := database.SelectContext(ctx, &puuids, `
		SELECT DISTINCT puuid
		FROM masteries
		ORDER BY puuid
		LIMIT ?
	`, options.SummonerSamples); err != nil {
		return result, fmt.Errorf("select mastery benchmark summoners: %w", err)
	}
	var championIDs []int64
	if err := database.SelectContext(ctx, &championIDs, `
		SELECT champion_id
		FROM mastery_statistics_aggregates
		ORDER BY summoner_count DESC, champion_id
		LIMIT ?
	`, options.ChampionSamples); err != nil {
		return result, fmt.Errorf("select mastery benchmark champions: %w", err)
	}
	if len(puuids) == 0 || len(championIDs) == 0 {
		return result, fmt.Errorf("mastery benchmark requires both summoner and champion samples")
	}

	result.SummonerSamples = len(puuids)
	result.ChampionSamples = len(championIDs)
	result.Iterations = options.Iterations
	result.ResultsEqual = true
	lookupLegacy, lookupNumeric := make([]time.Duration, 0, len(puuids)*options.Iterations), make([]time.Duration, 0, len(puuids)*options.Iterations)
	aggregateLegacy, aggregateNumeric := make([]time.Duration, 0, len(championIDs)*options.Iterations), make([]time.Duration, 0, len(championIDs)*options.Iterations)
	rankerLegacy, rankerNumeric := make([]time.Duration, 0, len(championIDs)*options.Iterations), make([]time.Duration, 0, len(championIDs)*options.Iterations)

	for iteration := 0; iteration <= options.Iterations; iteration++ {
		for _, puuid := range puuids {
			var legacyRows, numericRows []masteryBenchmarkRow
			legacyDuration, err := benchmarkSelect(ctx, database, &legacyRows, legacyMasteryLookupQuery, puuid)
			if err != nil {
				return result, fmt.Errorf("benchmark legacy mastery lookup: %w", err)
			}
			numericDuration, err := benchmarkSelect(ctx, database, &numericRows, numericMasteryLookupQuery, puuid)
			if err != nil {
				return result, fmt.Errorf("benchmark numeric mastery lookup: %w", err)
			}
			if !equalMasteryBenchmarkRows(legacyRows, numericRows) {
				result.ResultsEqual = false
			}
			if iteration > 0 {
				lookupLegacy = append(lookupLegacy, legacyDuration)
				lookupNumeric = append(lookupNumeric, numericDuration)
			}
		}
		for _, championID := range championIDs {
			var legacyAggregate, numericAggregate masteryBenchmarkAggregate
			legacyDuration, err := benchmarkGet(ctx, database, &legacyAggregate, legacyMasteryAggregateQuery, championID)
			if err != nil {
				return result, fmt.Errorf("benchmark legacy mastery aggregate: %w", err)
			}
			numericDuration, err := benchmarkGet(ctx, database, &numericAggregate, numericMasteryAggregateQuery, championID)
			if err != nil {
				return result, fmt.Errorf("benchmark numeric mastery aggregate: %w", err)
			}
			if legacyAggregate != numericAggregate {
				result.ResultsEqual = false
			}

			var legacyRankers, numericRankers []masteryBenchmarkRanker
			legacyRankerDuration, err := benchmarkSelect(ctx, database, &legacyRankers, legacyMasteryTopRankersQuery, championID)
			if err != nil {
				return result, fmt.Errorf("benchmark legacy mastery top rankers: %w", err)
			}
			numericRankerDuration, err := benchmarkSelect(ctx, database, &numericRankers, numericMasteryTopRankersQuery, championID)
			if err != nil {
				return result, fmt.Errorf("benchmark numeric mastery top rankers: %w", err)
			}
			if !equalMasteryBenchmarkRankers(legacyRankers, numericRankers) {
				result.ResultsEqual = false
			}
			if iteration > 0 {
				aggregateLegacy = append(aggregateLegacy, legacyDuration)
				aggregateNumeric = append(aggregateNumeric, numericDuration)
				rankerLegacy = append(rankerLegacy, legacyRankerDuration)
				rankerNumeric = append(rankerNumeric, numericRankerDuration)
			}
		}
	}

	result.SummonerLookup = benchmarkTimingPair(lookupLegacy, lookupNumeric)
	result.Aggregate = benchmarkTimingPair(aggregateLegacy, aggregateNumeric)
	result.TopRankers = benchmarkTimingPair(rankerLegacy, rankerNumeric)
	if !result.ResultsEqual {
		return result, fmt.Errorf("legacy and numeric mastery benchmark results differ: %s", result.String())
	}
	return result, nil
}

func benchmarkSelect(ctx context.Context, database *sqlx.DB, destination interface{}, query string, args ...interface{}) (time.Duration, error) {
	started := time.Now()
	err := database.SelectContext(ctx, destination, query, args...)
	return time.Since(started), err
}

func benchmarkGet(ctx context.Context, database *sqlx.DB, destination interface{}, query string, args ...interface{}) (time.Duration, error) {
	started := time.Now()
	err := database.GetContext(ctx, destination, query, args...)
	return time.Since(started), err
}

func equalMasteryBenchmarkRows(left, right []masteryBenchmarkRow) bool {
	if len(left) != len(right) {
		return false
	}
	sort.Slice(left, func(i, j int) bool { return left[i].ChampionID < left[j].ChampionID })
	sort.Slice(right, func(i, j int) bool { return right[i].ChampionID < right[j].ChampionID })
	for index := range left {
		legacy := left[index]
		numeric := right[index]
		// The legacy table intentionally keeps its transitional summoner_fk
		// nullable, while the compact table requires the numeric parent. It is
		// not part of the API payload, so compare only user-visible mastery data.
		legacy.SummonerFK = sql.NullInt64{}
		numeric.SummonerFK = sql.NullInt64{}
		if legacy != numeric {
			return false
		}
	}
	return true
}

func equalMasteryBenchmarkRankers(left, right []masteryBenchmarkRanker) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ChampionPoints != right[index].ChampionPoints {
			return false
		}
	}
	return true
}

func benchmarkTimingPair(legacy, numeric []time.Duration) masteryReadBenchmarkPair {
	legacyTiming := summarizeBenchmarkTimings(legacy)
	numericTiming := summarizeBenchmarkTimings(numeric)
	speedup := 0.0
	if numericTiming.P50Ms > 0 {
		speedup = legacyTiming.P50Ms / numericTiming.P50Ms
	}
	return masteryReadBenchmarkPair{Legacy: legacyTiming, NumericV2: numericTiming, Speedup: speedup}
}

func summarizeBenchmarkTimings(durations []time.Duration) masteryReadBenchmarkTiming {
	if len(durations) == 0 {
		return masteryReadBenchmarkTiming{}
	}
	sorted := append([]time.Duration(nil), durations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var total time.Duration
	for _, duration := range sorted {
		total += duration
	}
	percentile := func(value float64) time.Duration {
		index := int(float64(len(sorted)-1) * value)
		return sorted[index]
	}
	return masteryReadBenchmarkTiming{
		Samples: len(sorted),
		P50Ms:   float64(percentile(0.50).Microseconds()) / 1000,
		P95Ms:   float64(percentile(0.95).Microseconds()) / 1000,
		MeanMs:  float64(total.Microseconds()) / float64(len(sorted)) / 1000,
	}
}
