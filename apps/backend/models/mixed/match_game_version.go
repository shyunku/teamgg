package mixed

import (
	"github.com/hashicorp/go-version"
	"sort"
	"team.gg-server/libs/db"
)

type MatchGameVersionMXDAO struct {
	GameVersion      string `db:"game_version" json:"gameVersion"`
	GameShortVersion string `db:"game_short_version" json:"gameShortVersion"`
	Count            int    `db:"count" json:"count"`
}

func getMatchGameVersionMXDAOs(db db.Context) ([]MatchGameVersionMXDAO, error) {
	var matchGameVersions []MatchGameVersionMXDAO
	if err := db.Select(&matchGameVersions, `
		SELECT game_version, 
		       SUBSTRING_INDEX(game_version, '.', 2) AS game_short_version,
		       COUNT(*) AS count
		FROM matches
		WHERE game_version != ''
		GROUP BY game_version;
	`); err != nil {
		return nil, err
	}
	return matchGameVersions, nil
}

func getMatchGameVersionMXDAOs_byDescendingVersion(db db.Context) ([]MatchGameVersionMXDAO, error) {
	matchGameVersionMXDAOs, err := getMatchGameVersionMXDAOs(db)
	if err != nil {
		return nil, err
	}

	// sort by descending version
	sort.SliceStable(matchGameVersionMXDAOs, func(i, j int) bool {
		iVersion, err := version.NewVersion(matchGameVersionMXDAOs[i].GameVersion)
		if err != nil {
			return false
		}
		jVersion, err := version.NewVersion(matchGameVersionMXDAOs[j].GameVersion)
		if err != nil {
			return false
		}
		return iVersion.GreaterThan(jVersion)
	})
	return matchGameVersionMXDAOs, nil
}

func GetRecentMatchGameVersions_byDescendingShortVersion_withCount(db db.Context, count int) ([]string, []string, error) {
	if count == -1 {
		count = 100000
	}
	matchGameVersionMXDAOs, err := getMatchGameVersionMXDAOs_byDescendingVersion(db)
	if err != nil {
		return nil, nil, err
	}

	// get recent game versions
	recentMatchGameVersions := make([]string, 0)
	gameShortVersions := make([]string, 0)
	seen := make(map[string]bool)
	for _, matchGameVersionMXDAO := range matchGameVersionMXDAOs {
		if _, exists := seen[matchGameVersionMXDAO.GameShortVersion]; !exists {
			seen[matchGameVersionMXDAO.GameShortVersion] = true
		}
		if len(seen) > count {
			break
		}
		recentMatchGameVersions = append(recentMatchGameVersions, matchGameVersionMXDAO.GameVersion)
	}
	for shortVersion, _ := range seen {
		gameShortVersions = append(gameShortVersions, shortVersion)
	}
	return recentMatchGameVersions, gameShortVersions, nil
}
