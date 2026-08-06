package models

import (
	"team.gg-server/libs/db"
)

type MatchParticipantDetailDAO struct {
	MatchParticipantId string `db:"match_participant_id" json:"matchParticipantId"`

	MatchId string `db:"match_id" json:"matchId"`

	BaronKills  int `db:"baron_kills" json:"baronKills"`
	BountyLevel int `db:"bounty_level" json:"bountyLevel"`

	ChampionTransform    int `db:"champion_transform" json:"championTransform"`
	ConsumablesPurchased int `db:"consumables_purchased" json:"consumablesPurchased"`

	DamageDealtToBuildings  int `db:"damage_dealt_to_buildings" json:"damageDealtToBuildings"`   // 건물에 입힌 피해량
	DamageDealtToObjectives int `db:"damage_dealt_to_objectives" json:"damageDealtToObjectives"` // 목표물에 입힌 피해량
	DamageDealtToTurrets    int `db:"damage_dealt_to_turrets" json:"damageDealtToTurrets"`       // 포탑에 입힌 피해량
	DamageSelfMitigated     int `db:"damage_self_mitigated" json:"damageSelfMitigated"`          // 자신에 대한 피해 감소량

	DetectorWardsPlaced int `db:"detector_wards_placed" json:"detectorWardsPlaced"`
	DragonKills         int `db:"dragon_kills" json:"dragonKills"`

	PhysicalDamageDealt int `db:"physical_damage_dealt" json:"physicalDamageDealt"`
	MagicDamageDealt    int `db:"magic_damage_dealt" json:"magicDamageDealt"`
	TotalDamageDealt    int `db:"total_damage_dealt" json:"totalDamageDealt"`

	LargestCriticalStrike int `db:"largest_critical_strike" json:"largestCriticalStrike"`
	LargestKillingSpree   int `db:"largest_killing_spree" json:"largestKillingSpree"`
	LargestMultiKill      int `db:"largest_multi_kill" json:"largestMultiKill"`

	FirstTowerAssist bool `db:"first_tower_assist" json:"firstTowerAssist"`
	FirstTowerKill   bool `db:"first_tower_kill" json:"firstTowerKill"`

	InhibitorKills     int `db:"inhibitor_kills" json:"inhibitorKills"`
	InhibitorTakedowns int `db:"inhibitor_takedowns" json:"inhibitorTakedowns"`
	InhibitorsLost     int `db:"inhibitors_lost" json:"inhibitorsLost"`
	ItemsPurchased     int `db:"items_purchased" json:"itemsPurchased"`

	KillingSprees  int `db:"killing_sprees" json:"killingSprees"`
	NexusKills     int `db:"nexus_kills" json:"nexusKills"`
	NexusTakedowns int `db:"nexus_takedowns" json:"nexusTakedowns"`
	NexusLost      int `db:"nexus_lost" json:"nexusLost"`

	LongestTimeSpentLiving int `db:"longest_time_spent_living" json:"longestTimeSpentLiving"`

	ObjectiveStolen        int `db:"objective_stolen" json:"objectiveStolen"`
	ObjectiveStolenAssists int `db:"objective_stolen_assists" json:"objectiveStolenAssists"`

	SightWardsBoughtInGame  int `db:"sight_wards_bought_in_game" json:"sightWardsBoughtInGame"`
	VisionWardsBoughtInGame int `db:"vision_wards_bought_in_game" json:"visionWardsBoughtInGame"`

	SummonerId string `db:"summoner_id" json:"summonerId"`

	TimeCCingOthers int `db:"time_ccing_others" json:"timeCCingOthers"`
	TimePlayed      int `db:"time_played" json:"timePlayed"`

	TotalDamageShieldedOnTeammates int `db:"total_damage_shielded_on_teammates" json:"totalDamageShieldedOnTeammates"`
	TotalTimeSpentDead             int `db:"total_time_spent_dead" json:"totalTimeSpentDead"`
	TotalUnitsHealed               int `db:"total_units_healed" json:"totalUnitsHealed"`
	TrueDamageDealt                int `db:"true_damage_dealt" json:"trueDamageDealt"`

	TurretKills     int `db:"turret_kills" json:"turretKills"`
	TurretTakedowns int `db:"turret_takedowns" json:"turretTakedowns"`
	TurretsLost     int `db:"turrets_lost" json:"turretsLost"`

	UnrealKills int `db:"unreal_kills" json:"unrealKills"`
	WardsKilled int `db:"wards_killed" json:"wardsKilled"`
	WardsPlaced int `db:"wards_placed" json:"wardsPlaced"`
}

func (m *MatchParticipantDetailDAO) Insert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO match_participant_details
		    (match_participant_id, match_id, baron_kills, bounty_level, champion_transform,
		     consumables_purchased, damage_dealt_to_buildings, damage_dealt_to_objectives, 
		     damage_dealt_to_turrets, damage_self_mitigated, detector_wards_placed, 
		     dragon_kills, physical_damage_dealt, magic_damage_dealt, total_damage_dealt, 
		     largest_critical_strike, largest_killing_spree, largest_multi_kill, 
		     first_tower_assist, first_tower_kill, inhibitor_kills, inhibitor_takedowns, 
		     inhibitors_lost, items_purchased, killing_sprees, nexus_kills, nexus_takedowns, 
		     nexus_lost, longest_time_spent_living, objective_stolen, objective_stolen_assists, 
		     sight_wards_bought_in_game, vision_wards_bought_in_game, summoner_id, time_ccing_others, 
		     time_played, total_damage_shielded_on_teammates, total_time_spent_dead, 
		     total_units_healed, true_damage_dealt, turret_kills, turret_takedowns, 
		     turrets_lost, unreal_kills, wards_killed, wards_placed)
		VALUE
		    (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.MatchParticipantId, m.MatchId, m.BaronKills, m.BountyLevel, m.ChampionTransform,
		m.ConsumablesPurchased, m.DamageDealtToBuildings, m.DamageDealtToObjectives,
		m.DamageDealtToTurrets, m.DamageSelfMitigated, m.DetectorWardsPlaced, m.DragonKills,
		m.PhysicalDamageDealt, m.MagicDamageDealt, m.TotalDamageDealt, m.LargestCriticalStrike,
		m.LargestKillingSpree, m.LargestMultiKill, m.FirstTowerAssist, m.FirstTowerKill,
		m.InhibitorKills, m.InhibitorTakedowns, m.InhibitorsLost, m.ItemsPurchased,
		m.KillingSprees, m.NexusKills, m.NexusTakedowns, m.NexusLost, m.LongestTimeSpentLiving,
		m.ObjectiveStolen, m.ObjectiveStolenAssists, m.SightWardsBoughtInGame, m.VisionWardsBoughtInGame,
		m.SummonerId, m.TimeCCingOthers, m.TimePlayed, m.TotalDamageShieldedOnTeammates, m.TotalTimeSpentDead,
		m.TotalUnitsHealed, m.TrueDamageDealt, m.TurretKills, m.TurretTakedowns,
		m.TurretsLost, m.UnrealKills, m.WardsKilled, m.WardsPlaced,
	); err != nil {
		return err
	}
	return nil
}

func GetMatchParticipantDetailDAOs_byMatchId(db db.Context, matchId string) ([]MatchParticipantDetailDAO, error) {
	var details []MatchParticipantDetailDAO
	if err := db.Select(&details, `
		SELECT *
		FROM match_participant_details
		WHERE match_id = ?`, matchId,
	); err != nil {
		return nil, err
	}
	return details, nil
}
