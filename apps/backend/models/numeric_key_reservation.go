package models

import (
	"fmt"
	"sort"
	"strings"

	"team.gg-server/libs/db"
)

// ReserveMatchNumericKeys acquires mapping rows in one deterministic order.
// Match persistence runs in parallel, so participant-order upserts can
// otherwise deadlock when two transactions touch overlapping PUUID sets.
func ReserveMatchNumericKeys(database db.Context, matchId string, puuids []string) error {
	if matchId != "" {
		if _, err := database.Exec(`
			INSERT INTO match_numeric_keys (riot_match_id) VALUES (?)
			ON DUPLICATE KEY UPDATE match_id = LAST_INSERT_ID(match_id)
		`, matchId); err != nil {
			return fmt.Errorf("reserve match numeric key: %w", err)
		}
	}

	unique := make(map[string]struct{}, len(puuids))
	ordered := make([]string, 0, len(puuids))
	for _, puuid := range puuids {
		if puuid == "" {
			continue
		}
		if _, exists := unique[puuid]; exists {
			continue
		}
		unique[puuid] = struct{}{}
		ordered = append(ordered, puuid)
	}
	if len(ordered) == 0 {
		return nil
	}
	sort.Strings(ordered)

	placeholders := make([]string, len(ordered))
	args := make([]interface{}, len(ordered))
	for index, puuid := range ordered {
		placeholders[index] = "(?)"
		args[index] = puuid
	}
	query := `INSERT INTO summoner_numeric_keys (puuid) VALUES ` +
		strings.Join(placeholders, ",") +
		` ON DUPLICATE KEY UPDATE summoner_id = LAST_INSERT_ID(summoner_id)`
	if _, err := database.Exec(query, args...); err != nil {
		return fmt.Errorf("reserve summoner numeric keys: %w", err)
	}
	return nil
}
