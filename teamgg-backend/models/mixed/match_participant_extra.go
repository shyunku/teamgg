package mixed

import (
	"database/sql"
	"errors"
	"math"
	"team.gg-server/libs/db"
)

type MatchParticipantExtraMXDAO struct {
	// match
	MatchId            string `db:"match_id" json:"matchId"`
	DataVersion        string `db:"data_version" json:"dataVersion"`
	GameCreation       int64  `db:"game_creation" json:"gameCreation"`
	GameDuration       int64  `db:"game_duration" json:"gameDuration"`
	GameEndTimestamp   int64  `db:"game_end_timestamp" json:"gameEndTimestamp"`
	GameId             int64  `db:"game_id" json:"gameId"`
	GameMode           string `db:"game_mode" json:"gameMode"`
	GameName           string `db:"game_name" json:"gameName"`
	GameStartTimestamp int64  `db:"game_start_timestamp" json:"gameStartTimestamp"`
	GameType           string `db:"game_type" json:"gameType"`
	GameVersion        string `db:"game_version" json:"gameVersion"`
	MapId              int    `db:"map_id" json:"mapId"`
	PlatformId         string `db:"platform_id" json:"platformId"`
	QueueId            int    `db:"queue_id" json:"queueId"`
	TournamentCode     string `db:"tournament_code" json:"tournamentCode"`

	// participant
	None0                          *string `db:"match_id" json:"none0"`
	ParticipantId                  int     `db:"participant_id" json:"participantId"`
	MatchParticipantId             string  `db:"match_participant_id" json:"matchParticipantId"`
	Puuid                          string  `db:"puuid" json:"puuid"`
	Kills                          int     `db:"kills" json:"kills"`
	Deaths                         int     `db:"deaths" json:"deaths"`
	Assists                        int     `db:"assists" json:"assists"`
	ChampionId                     int     `db:"champion_id" json:"championId"`
	ChampionLevel                  int     `db:"champion_level" json:"championLevel"`
	ChampionName                   string  `db:"champion_name" json:"championName"`
	ChampExperience                int     `db:"champ_experience" json:"champExperience"`
	SummonerLevel                  int     `db:"summoner_level" json:"summonerLevel"`
	SummonerName                   string  `db:"summoner_name" json:"summonerName"`
	RiotIdName                     string  `db:"riot_id_name" json:"riotIdName"`
	RiotIdTagLine                  string  `db:"riot_id_tag_line" json:"riotIdTagLine"`
	ProfileIcon                    int     `db:"profile_icon" json:"profileIcon"`
	MagicDamageDealtToChampions    int     `db:"magic_damage_dealt_to_champions" json:"magicDamageDealtToChampions"`
	PhysicalDamageDealtToChampions int     `db:"physical_damage_dealt_to_champions" json:"physicalDamageDealtToChampions"`
	TrueDamageDealtToChampions     int     `db:"true_damage_dealt_to_champions" json:"trueDamageDealtToChampions"`
	TotalDamageDealtToChampions    int     `db:"total_damage_dealt_to_champions" json:"totalDamageDealtToChampions"`
	MagicDamageTaken               int     `db:"magic_damage_taken" json:"magicDamageTaken"`
	PhysicalDamageTaken            int     `db:"physical_damage_taken" json:"physicalDamageTaken"`
	TrueDamageTaken                int     `db:"true_damage_taken" json:"trueDamageTaken"`
	TotalDamageTaken               int     `db:"total_damage_taken" json:"totalDamageTaken"`
	TotalHeal                      int     `db:"total_heal" json:"totalHeal"`
	TotalHealsOnTeammates          int     `db:"total_heals_on_teammates" json:"totalHealsOnTeammates"`
	Item0                          int     `db:"item0" json:"item0"`
	Item1                          int     `db:"item1" json:"item1"`
	Item2                          int     `db:"item2" json:"item2"`
	Item3                          int     `db:"item3" json:"item3"`
	Item4                          int     `db:"item4" json:"item4"`
	Item5                          int     `db:"item5" json:"item5"`
	Item6                          int     `db:"item6" json:"item6"`
	Spell1Casts                    int     `db:"spell1_casts" json:"spell1Casts"`
	Spell2Casts                    int     `db:"spell2_casts" json:"spell2Casts"`
	Spell3Casts                    int     `db:"spell3_casts" json:"spell3Casts"`
	Spell4Casts                    int     `db:"spell4_casts" json:"spell4Casts"`
	Summoner1Casts                 int     `db:"summoner1_casts" json:"summoner1Casts"`
	Summoner1Id                    int     `db:"summoner1_id" json:"summoner1Id"`
	Summoner2Casts                 int     `db:"summoner2_casts" json:"summoner2Casts"`
	Summoner2Id                    int     `db:"summoner2_id" json:"summoner2Id"`
	FirstBloodAssist               bool    `db:"first_blood_assist" json:"firstBloodAssist"`
	FirstBloodKill                 bool    `db:"first_blood_kill" json:"firstBloodKill"`
	DoubleKills                    int     `db:"double_kills" json:"doubleKills"`
	TripleKills                    int     `db:"triple_kills" json:"tripleKills"`
	QuadraKills                    int     `db:"quadra_kills" json:"quadraKills"`
	PentaKills                     int     `db:"penta_kills" json:"pentaKills"`
	TotalMinionsKilled             int     `db:"total_minions_killed" json:"totalMinionsKilled"`
	TotalTimeCCDealt               int     `db:"total_time_cc_dealt" json:"totalTimeCCDealt"`
	NeutralMinionsKilled           int     `db:"neutral_minions_killed" json:"neutralMinionsKilled"`
	GoldSpent                      int     `db:"gold_spent" json:"goldSpent"`
	GoldEarned                     int     `db:"gold_earned" json:"goldEarned"`
	IndividualPosition             string  `db:"individual_position" json:"individualPosition"`
	TeamPosition                   string  `db:"team_position" json:"teamPosition"`
	Lane                           string  `db:"lane" json:"lane"`
	Role                           string  `db:"role" json:"role"`
	TeamId                         int     `db:"team_id" json:"teamId"`
	VisionScore                    int     `db:"vision_score" json:"visionScore"`
	Win                            bool    `db:"win" json:"win"`
	GameEndedInEarlySurrender      bool    `db:"game_ended_in_early_surrender" json:"gameEndedInEarlySurrender"`
	GameEndedInSurrender           bool    `db:"game_ended_in_surrender" json:"gameEndedInSurrender"`
	TeamEarlySurrendered           bool    `db:"team_early_surrendered" json:"teamEarlySurrendered"`

	//	Details
	None1                          *string `db:"mpd.match_participant_id" json:"none1"`
	None2                          *string `db:"mpd.match_id" json:"none2"`
	BaronKills                     int     `db:"baron_kills" json:"baronKills"`
	BountyLevel                    int     `db:"bounty_level" json:"bountyLevel"`
	ChampionTransform              int     `db:"champion_transform" json:"championTransform"`
	ConsumablesPurchased           int     `db:"consumables_purchased" json:"consumablesPurchased"`
	DamageDealtToBuildings         int     `db:"damage_dealt_to_buildings" json:"damageDealtToBuildings"`   // 건물에 입힌 피해량
	DamageDealtToObjectives        int     `db:"damage_dealt_to_objectives" json:"damageDealtToObjectives"` // 목표물에 입힌 피해량
	DamageDealtToTurrets           int     `db:"damage_dealt_to_turrets" json:"damageDealtToTurrets"`       // 포탑에 입힌 피해량
	DamageSelfMitigated            int     `db:"damage_self_mitigated" json:"damageSelfMitigated"`          // 자신에 대한 피해 감소량
	DetectorWardsPlaced            int     `db:"detector_wards_placed" json:"detectorWardsPlaced"`
	DragonKills                    int     `db:"dragon_kills" json:"dragonKills"`
	PhysicalDamageDealt            int     `db:"physical_damage_dealt" json:"physicalDamageDealt"`
	MagicDamageDealt               int     `db:"magic_damage_dealt" json:"magicDamageDealt"`
	TotalDamageDealt               int     `db:"total_damage_dealt" json:"totalDamageDealt"`
	LargestCriticalStrike          int     `db:"largest_critical_strike" json:"largestCriticalStrike"`
	LargestKillingSpree            int     `db:"largest_killing_spree" json:"largestKillingSpree"`
	LargestMultiKill               int     `db:"largest_multi_kill" json:"largestMultiKill"`
	FirstTowerAssist               bool    `db:"first_tower_assist" json:"firstTowerAssist"`
	FirstTowerKill                 bool    `db:"first_tower_kill" json:"firstTowerKill"`
	InhibitorKills                 int     `db:"inhibitor_kills" json:"inhibitorKills"`
	InhibitorTakedowns             int     `db:"inhibitor_takedowns" json:"inhibitorTakedowns"`
	InhibitorsLost                 int     `db:"inhibitors_lost" json:"inhibitorsLost"`
	ItemsPurchased                 int     `db:"items_purchased" json:"itemsPurchased"`
	KillingSprees                  int     `db:"killing_sprees" json:"killingSprees"`
	NexusKills                     int     `db:"nexus_kills" json:"nexusKills"`
	NexusTakedowns                 int     `db:"nexus_takedowns" json:"nexusTakedowns"`
	NexusLost                      int     `db:"nexus_lost" json:"nexusLost"`
	LongestTimeSpentLiving         int     `db:"longest_time_spent_living" json:"longestTimeSpentLiving"`
	ObjectiveStolen                int     `db:"objective_stolen" json:"objectiveStolen"`
	ObjectiveStolenAssists         int     `db:"objective_stolen_assists" json:"objectiveStolenAssists"`
	SightWardsBoughtInGame         int     `db:"sight_wards_bought_in_game" json:"sightWardsBoughtInGame"`
	VisionWardsBoughtInGame        int     `db:"vision_wards_bought_in_game" json:"visionWardsBoughtInGame"`
	SummonerId                     string  `db:"summoner_id" json:"summonerId"`
	TimeCCingOthers                int     `db:"time_ccing_others" json:"timeCCingOthers"`
	TimePlayed                     int     `db:"time_played" json:"timePlayed"`
	TotalDamageShieldedOnTeammates int     `db:"total_damage_shielded_on_teammates" json:"totalDamageShieldedOnTeammates"`
	TotalTimeSpentDead             int     `db:"total_time_spent_dead" json:"totalTimeSpentDead"`
	TotalUnitsHealed               int     `db:"total_units_healed" json:"totalUnitsHealed"`
	TrueDamageDealt                int     `db:"true_damage_dealt" json:"trueDamageDealt"`
	TurretKills                    int     `db:"turret_kills" json:"turretKills"`
	TurretTakedowns                int     `db:"turret_takedowns" json:"turretTakedowns"`
	TurretsLost                    int     `db:"turrets_lost" json:"turretsLost"`
	UnrealKills                    int     `db:"unreal_kills" json:"unrealKills"`
	WardsKilled                    int     `db:"wards_killed" json:"wardsKilled"`
	WardsPlaced                    int     `db:"wards_placed" json:"wardsPlaced"`
}

func (m *MatchParticipantExtraMXDAO) GetScore() float64 {
	kdaCutLine := 30
	killsCutLine := 30
	damageCutLine := 80000
	healCutLine := 50000
	objectCutLine := 30000
	tankerCutLine := 150000
	wardCutLine := 120
	ccCutLine := 3600

	gameDurationFactor := 3600 / float64(m.GameDuration)

	var (
		kdaScore         = 0.0
		killScore        = m.Kills
		damageScore      = m.TotalDamageDealtToChampions
		healScore        = m.TotalHealsOnTeammates
		objectDealtScore = m.DamageDealtToBuildings + m.DamageDealtToTurrets
		tankerScore      = float64(m.TotalDamageTaken)*0.5 + float64(m.DamageSelfMitigated)
		wardScore        = m.VisionScore
		ccScore          = m.TotalTimeCCDealt // apply average cc duration for champion
	)

	if m.Deaths == 0 {
		kdaScore = float64(m.Kills+m.Assists) * 1.2
	} else {
		kdaScore = float64(m.Kills+m.Assists) / float64(m.Deaths)
	}

	var (
		kdaPart    = float64(kdaScore) / float64(kdaCutLine)
		killPart   = float64(killScore) / float64(killsCutLine)
		damagePart = float64(damageScore) / float64(damageCutLine)
		healPart   = float64(healScore) / float64(healCutLine)
		objectPart = float64(objectDealtScore) / float64(objectCutLine)
		tankerPart = float64(tankerScore) / float64(tankerCutLine)
		wardPart   = float64(wardScore) / float64(wardCutLine)
		ccPart     = float64(ccScore) / float64(ccCutLine)
	)

	parts := []float64{kdaPart, killPart, damagePart, healPart, objectPart, tankerPart, wardPart, ccPart}

	totalScore := 0.0
	for _, part := range parts {
		totalScore += part
	}
	finalScore := (100 * (totalScore * gameDurationFactor)) / float64(len(parts))
	if m.GameDuration < 300 {
		finalScore = math.Sqrt(finalScore)
	}

	return finalScore
}

func getRecentMatchParticipantExtraMXDAOs(puuid string, count int) ([]MatchParticipantExtraMXDAO, error) {
	var details []MatchParticipantExtraMXDAO
	if err := db.Root.Select(&details, `
		SELECT m.*, mp.*, mpd.*
		FROM summoners s
		LEFT JOIN match_participants mp ON s.puuid = mp.puuid
		LEFT JOIN match_participant_details mpd ON mp.match_participant_id = mpd.match_participant_id
		LEFT JOIN matches m on m.match_id = mp.match_id
		WHERE s.puuid = ?
		ORDER BY m.game_end_timestamp DESC
		LIMIT ?;
	`, puuid, count); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]MatchParticipantExtraMXDAO, 0), nil
		}
		return nil, err
	}
	return details, nil
}

//func GetMatchParticipantExtraMXDAO_byMatchId(matchId, puuid string) (*MatchParticipantExtraMXDAO, error) {
//	var detail MatchParticipantExtraMXDAO
//	if err := db.Root.Get(&detail, `
//		SELECT m.*, mp.*, mpd.*
//		FROM matches m
//		LEFT JOIN match_participants mp ON mp.match_id = m.match_id
//		LEFT JOIN match_participant_details mpd ON mp.match_participant_id = mpd.match_participant_id
//		WHERE m.match_id = ? AND puuid = ?
//		ORDER BY m.game_end_timestamp DESC
//		LIMIT 1;
//	`, matchId, puuid); err != nil {
//		return nil, err
//	}
//	return &detail, nil
//}

func GetMatchParticipantExtraMXDAOs_byMatchId(matchId string) ([]MatchParticipantExtraMXDAO, error) {
	//if core.DebugOnProd {
	//	defer util.InspectFunctionExecutionTime()()
	//}
	var details []MatchParticipantExtraMXDAO
	if err := db.Root.Select(&details, `
		SELECT m.*, mp.*, mpd.*
		FROM matches m
		LEFT JOIN match_participants mp ON mp.match_id = m.match_id
		LEFT JOIN match_participant_details mpd ON mp.match_participant_id = mpd.match_participant_id
		WHERE m.match_id = ?
		ORDER BY m.game_end_timestamp DESC;
	`, matchId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]MatchParticipantExtraMXDAO, 0), nil
		}
		return nil, err
	}
	return details, nil
}

func GetMatchParticipantExtraMXDAOs_byQueueId(puuid string, queueId, count int) ([]MatchParticipantExtraMXDAO, error) {
	if queueId == 0 {
		return getRecentMatchParticipantExtraMXDAOs(puuid, count)
	}
	var details []MatchParticipantExtraMXDAO
	if err := db.Root.Select(&details, `
		SELECT m.*, mp.*, mpd.*
		FROM summoners s
		LEFT JOIN match_participants mp ON s.puuid = mp.puuid
		LEFT JOIN match_participant_details mpd ON mp.match_participant_id = mpd.match_participant_id
		LEFT JOIN matches m on m.match_id = mp.match_id
		WHERE s.puuid = ? AND m.queue_id = ?
		ORDER BY m.game_end_timestamp DESC
		LIMIT ?;
	`, puuid, queueId, count); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]MatchParticipantExtraMXDAO, 0), nil
		}
		return nil, err
	}
	return details, nil
}
