package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type MatchParticipantDAO struct {
	MatchId            string `db:"match_id" json:"matchId"`
	ParticipantId      int    `db:"participant_id" json:"participantId"`
	MatchParticipantId string `db:"match_participant_id" json:"matchParticipantId"`

	MatchParticipantPk sql.NullInt64 `db:"match_participant_pk" json:"-"`
	MatchFk            sql.NullInt64 `db:"match_fk" json:"-"`
	SummonerFk         sql.NullInt64 `db:"summoner_fk" json:"-"`

	Puuid string `db:"puuid" json:"puuid"`

	Kills   int `db:"kills" json:"kills"`
	Deaths  int `db:"deaths" json:"deaths"`
	Assists int `db:"assists" json:"assists"`

	ChampionId      int    `db:"champion_id" json:"championId"`
	ChampionLevel   int    `db:"champion_level" json:"championLevel"`
	ChampionName    string `db:"champion_name" json:"championName"`
	ChampExperience int    `db:"champ_experience" json:"champExperience"`

	SummonerLevel int    `db:"summoner_level" json:"summonerLevel"`
	SummonerName  string `db:"summoner_name" json:"summonerName"`
	RiotIdName    string `db:"riot_id_name" json:"riotIdName"`
	RiotIdTagLine string `db:"riot_id_tag_line" json:"riotIdTagLine"`
	ProfileIcon   int    `db:"profile_icon" json:"profileIcon"`

	MagicDamageDealtToChampions    int `db:"magic_damage_dealt_to_champions" json:"magicDamageDealtToChampions"`
	PhysicalDamageDealtToChampions int `db:"physical_damage_dealt_to_champions" json:"physicalDamageDealtToChampions"`
	TrueDamageDealtToChampions     int `db:"true_damage_dealt_to_champions" json:"trueDamageDealtToChampions"`
	TotalDamageDealtToChampions    int `db:"total_damage_dealt_to_champions" json:"totalDamageDealtToChampions"`

	MagicDamageTaken    int `db:"magic_damage_taken" json:"magicDamageTaken"`
	PhysicalDamageTaken int `db:"physical_damage_taken" json:"physicalDamageTaken"`
	TrueDamageTaken     int `db:"true_damage_taken" json:"trueDamageTaken"`
	TotalDamageTaken    int `db:"total_damage_taken" json:"totalDamageTaken"`

	TotalHeal             int `db:"total_heal" json:"totalHeal"`
	TotalHealsOnTeammates int `db:"total_heals_on_teammates" json:"totalHealsOnTeammates"`

	Item0 int `db:"item0" json:"item0"`
	Item1 int `db:"item1" json:"item1"`
	Item2 int `db:"item2" json:"item2"`
	Item3 int `db:"item3" json:"item3"`
	Item4 int `db:"item4" json:"item4"`
	Item5 int `db:"item5" json:"item5"`
	Item6 int `db:"item6" json:"item6"`

	Spell1Casts int `db:"spell1_casts" json:"spell1Casts"`
	Spell2Casts int `db:"spell2_casts" json:"spell2Casts"`
	Spell3Casts int `db:"spell3_casts" json:"spell3Casts"`
	Spell4Casts int `db:"spell4_casts" json:"spell4Casts"`

	Summoner1Casts int `db:"summoner1_casts" json:"summoner1Casts"`
	Summoner1Id    int `db:"summoner1_id" json:"summoner1Id"`
	Summoner2Casts int `db:"summoner2_casts" json:"summoner2Casts"`
	Summoner2Id    int `db:"summoner2_id" json:"summoner2Id"`

	FirstBloodAssist bool `db:"first_blood_assist" json:"firstBloodAssist"`
	FirstBloodKill   bool `db:"first_blood_kill" json:"firstBloodKill"`

	DoubleKills int `db:"double_kills" json:"doubleKills"`
	TripleKills int `db:"triple_kills" json:"tripleKills"`
	QuadraKills int `db:"quadra_kills" json:"quadraKills"`
	PentaKills  int `db:"penta_kills" json:"pentaKills"`

	TotalMinionsKilled   int `db:"total_minions_killed" json:"totalMinionsKilled"`
	TotalTimeCCDealt     int `db:"total_time_cc_dealt" json:"totalTimeCCDealt"`
	NeutralMinionsKilled int `db:"neutral_minions_killed" json:"neutralMinionsKilled"`

	GoldSpent  int `db:"gold_spent" json:"goldSpent"`
	GoldEarned int `db:"gold_earned" json:"goldEarned"`

	IndividualPosition string `db:"individual_position" json:"individualPosition"`
	TeamPosition       string `db:"team_position" json:"teamPosition"`
	Lane               string `db:"lane" json:"lane"`
	Role               string `db:"role" json:"role"`
	TeamId             int    `db:"team_id" json:"teamId"`

	VisionScore int `db:"vision_score" json:"visionScore"`

	Win bool `db:"win" json:"win"`

	GameEndedInEarlySurrender bool `db:"game_ended_in_early_surrender" json:"gameEndedInEarlySurrender"`
	GameEndedInSurrender      bool `db:"game_ended_in_surrender" json:"gameEndedInSurrender"`
	TeamEarlySurrendered      bool `db:"team_early_surrendered" json:"teamEarlySurrendered"`
}

func (m *MatchParticipantDAO) Insert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO match_participants
		    (match_id, participant_id, match_participant_id, puuid, kills, deaths, assists, 
		     champion_id, champion_level, champion_name, champ_experience, summoner_level, 
		     summoner_name, riot_id_name, riot_id_tag_line, profile_icon, 
		     magic_damage_dealt_to_champions, physical_damage_dealt_to_champions, 
		     true_damage_dealt_to_champions, total_damage_dealt_to_champions, 
		     magic_damage_taken, physical_damage_taken, true_damage_taken, 
		     total_damage_taken, total_heal, total_heals_on_teammates, 
		     item0, item1, item2, item3, item4, item5, item6, 
		     spell1_casts, spell2_casts, spell3_casts, spell4_casts, 
		     summoner1_casts, summoner1_id, summoner2_casts, summoner2_id, 
		     first_blood_assist, first_blood_kill, double_kills, triple_kills, 
		     quadra_kills, penta_kills, total_minions_killed, total_time_cc_dealt, 
		     neutral_minions_killed, gold_spent, gold_earned, individual_position, 
		     team_position, lane, role, team_id, vision_score, win, game_ended_in_early_surrender, 
		     game_ended_in_surrender, team_early_surrendered) 
		VALUE
		    (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.MatchId, m.ParticipantId, m.MatchParticipantId, m.Puuid, m.Kills, m.Deaths, m.Assists,
		m.ChampionId, m.ChampionLevel, m.ChampionName, m.ChampExperience, m.SummonerLevel, m.SummonerName,
		m.RiotIdName, m.RiotIdTagLine, m.ProfileIcon, m.MagicDamageDealtToChampions, m.PhysicalDamageDealtToChampions,
		m.TrueDamageDealtToChampions, m.TotalDamageDealtToChampions, m.MagicDamageTaken,
		m.PhysicalDamageTaken, m.TrueDamageTaken, m.TotalDamageTaken, m.TotalHeal, m.TotalHealsOnTeammates,
		m.Item0, m.Item1, m.Item2, m.Item3, m.Item4, m.Item5, m.Item6,
		m.Spell1Casts, m.Spell2Casts, m.Spell3Casts, m.Spell4Casts,
		m.Summoner1Casts, m.Summoner1Id, m.Summoner2Casts, m.Summoner2Id,
		m.FirstBloodAssist, m.FirstBloodKill, m.DoubleKills, m.TripleKills, m.QuadraKills, m.PentaKills,
		m.TotalMinionsKilled, m.TotalTimeCCDealt, m.NeutralMinionsKilled,
		m.GoldSpent, m.GoldEarned, m.IndividualPosition, m.TeamPosition,
		m.Lane, m.Role, m.TeamId, m.VisionScore, m.Win,
		m.GameEndedInEarlySurrender, m.GameEndedInSurrender, m.TeamEarlySurrendered,
	); err != nil {
		return err
	}
	return nil
}

func GetMatchParticipantDAOs(db db.Context, matchId string) ([]MatchParticipantDAO, error) {
	var matchParticipants []MatchParticipantDAO
	if err := db.Select(&matchParticipants,
		"SELECT * FROM match_participants WHERE match_id = ?", matchId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]MatchParticipantDAO, 0), nil
		}
		return nil, err
	}
	return matchParticipants, nil
}
