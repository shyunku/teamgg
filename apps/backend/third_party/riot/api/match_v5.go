package api

import (
	"encoding/json"
	"team.gg-server/libs/http"
	"team.gg-server/third_party/riot"
	"time"
)

type MatchIdsDto []string

type MatchIdsReqOption struct {
	QueueId   int
	MatchType *string
	Count     int
	StartTime *time.Time
	EndTime   *time.Time
}

func GetMatchIdsInterval(puuid string, opt *MatchIdsReqOption) (*MatchIdsDto, error) {
	riot.UpdateRiotApiCalls()
	query := make(map[string]interface{})
	if opt != nil {
		if opt.StartTime != nil {
			query["startTime"] = opt.StartTime.Unix()
		}
		if opt.EndTime != nil {
			query["endTime"] = opt.EndTime.Unix()
		}
		if opt.QueueId != 0 {
			query["queue"] = opt.QueueId
		}
		if opt.Count != 0 {
			query["count"] = opt.Count
		}
	}
	resp, err := http.Get(http.GetRequest{
		Url: riot.CreateUrlWithQuery(riot.RegionAsia, "/lol/match/v5/matches/by-puuid/"+puuid+"/ids", query),
	})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Err
	}

	var matches MatchIdsDto
	if err := json.Unmarshal(resp.Body, &matches); err != nil {
		return nil, err
	}

	return &matches, nil
}

type MatchDto struct {
	Metadata struct {
		DataVersion  string   `json:"dataVersion"`
		MatchId      string   `json:"matchId"`
		Participants []string `json:"participants"`
	} `json:"metadata"`
	Info struct {
		GameCreation       int64  `json:"gameCreation"`
		GameDuration       int64  `json:"gameDuration"`
		GameEndTimestamp   int64  `json:"gameEndTimestamp"`
		GameId             int64  `json:"gameId"`
		GameMode           string `json:"gameMode"`
		GameName           string `json:"gameName"`
		GameStartTimestamp int64  `json:"gameStartTimestamp"`
		GameType           string `json:"gameType"`
		GameVersion        string `json:"gameVersion"`
		MapId              int    `json:"mapId"`
		Participants       []struct {
			Assists                     int    `json:"assists"`
			BaronKills                  int    `json:"baronKills"`
			BountyLevel                 int    `json:"bountyLevel"`
			ChampExperience             int    `json:"champExperience"`
			ChampLevel                  int    `json:"champLevel"`
			ChampionId                  int    `json:"championId"`
			ChampionName                string `json:"championName"`
			ChampionTransform           int    `json:"championTransform"`
			ConsumablesPurchased        int    `json:"consumablesPurchased"`
			DamageDealtToBuildings      int    `json:"damageDealtToBuildings"`
			DamageDealtToObjectives     int    `json:"damageDealtToObjectives"`
			DamageDealtToTurrets        int    `json:"damageDealtToTurrets"`
			DamageSelfMitigated         int    `json:"damageSelfMitigated"`
			Deaths                      int    `json:"deaths"`
			DetectorWardsPlaced         int    `json:"detectorWardsPlaced"`
			DoubleKills                 int    `json:"doubleKills"`
			DragonKills                 int    `json:"dragonKills"`
			FirstBloodAssist            bool   `json:"firstBloodAssist"`
			FirstBloodKill              bool   `json:"firstBloodKill"`
			FirstTowerAssist            bool   `json:"firstTowerAssist"`
			FirstTowerKill              bool   `json:"firstTowerKill"`
			GameEndedInEarlySurrender   bool   `json:"gameEndedInEarlySurrender"`
			GameEndedInSurrender        bool   `json:"gameEndedInSurrender"`
			GoldEarned                  int    `json:"goldEarned"`
			GoldSpent                   int    `json:"goldSpent"`
			IndividualPosition          string `json:"individualPosition"`
			InhibitorKills              int    `json:"inhibitorKills"`
			InhibitorTakedowns          int    `json:"inhibitorTakedowns"`
			InhibitorsLost              int    `json:"inhibitorsLost"`
			Item0                       int    `json:"item0"`
			Item1                       int    `json:"item1"`
			Item2                       int    `json:"item2"`
			Item3                       int    `json:"item3"`
			Item4                       int    `json:"item4"`
			Item5                       int    `json:"item5"`
			Item6                       int    `json:"item6"`
			ItemsPurchased              int    `json:"itemsPurchased"`
			KillingSprees               int    `json:"killingSprees"`
			Kills                       int    `json:"kills"`
			Lane                        string `json:"lane"`
			LargestCriticalStrike       int    `json:"largestCriticalStrike"`
			LargestKillingSpree         int    `json:"largestKillingSpree"`
			LargestMultiKill            int    `json:"largestMultiKill"`
			LongestTimeSpentLiving      int    `json:"longestTimeSpentLiving"`
			MagicDamageDealt            int    `json:"magicDamageDealt"`
			MagicDamageDealtToChampions int    `json:"magicDamageDealtToChampions"`
			MagicDamageTaken            int    `json:"magicDamageTaken"`
			NeutralMinionsKilled        int    `json:"neutralMinionsKilled"`
			NexusKills                  int    `json:"nexusKills"`
			NexusTakedowns              int    `json:"nexusTakedowns"`
			NexusLost                   int    `json:"nexusLost"`
			ObjectiveStolen             int    `json:"objectiveStolen"`
			ObjectiveStolenAssists      int    `json:"objectiveStolenAssists"`
			ParticipantId               int    `json:"participantId"`
			PentaKills                  int    `json:"pentaKills"`
			Perks                       struct {
				StatPerks struct {
					Defense int `json:"defense"`
					Flex    int `json:"flex"`
					Offense int `json:"offense"`
				} `json:"statPerks"`
				Styles []struct {
					Description string `json:"description"`
					Selections  []struct {
						Perk int `json:"perk"`
						Var1 int `json:"var1"`
						Var2 int `json:"var2"`
						Var3 int `json:"var3"`
					} `json:"selections"`
					Style int `json:"style"`
				} `json:"styles"`
			} `json:"perks"`
			PhysicalDamageDealt            int    `json:"physicalDamageDealt"`
			PhysicalDamageDealtToChampions int    `json:"physicalDamageDealtToChampions"`
			PhysicalDamageTaken            int    `json:"physicalDamageTaken"`
			ProfileIcon                    int    `json:"profileIcon"`
			Puuid                          string `json:"puuid"`
			QuadraKills                    int    `json:"quadraKills"`
			RiotIdGameName                 string `json:"riotIdGameName"`
			RiotIdTagline                  string `json:"riotIdTagline"`
			Role                           string `json:"role"`
			SightWardsBoughtInGame         int    `json:"sightWardsBoughtInGame"`
			Spell1Casts                    int    `json:"spell1Casts"`
			Spell2Casts                    int    `json:"spell2Casts"`
			Spell3Casts                    int    `json:"spell3Casts"`
			Spell4Casts                    int    `json:"spell4Casts"`
			Summoner1Casts                 int    `json:"summoner1Casts"`
			Summoner1Id                    int    `json:"summoner1Id"`
			Summoner2Casts                 int    `json:"summoner2Casts"`
			Summoner2Id                    int    `json:"summoner2Id"`
			SummonerId                     string `json:"summonerId"`
			SummonerLevel                  int    `json:"summonerLevel"`
			SummonerName                   string `json:"summonerName"`
			TeamEarlySurrendered           bool   `json:"teamEarlySurrendered"`
			TeamId                         int    `json:"teamId"`
			TeamPosition                   string `json:"teamPosition"`
			TimeCCingOthers                int    `json:"timeCCingOthers"`
			TimePlayed                     int    `json:"timePlayed"`
			TotalDamageDealt               int    `json:"totalDamageDealt"`
			TotalDamageDealtToChampions    int    `json:"totalDamageDealtToChampions"`
			TotalDamageShieldedOnTeammates int    `json:"totalDamageShieldedOnTeammates"`
			TotalDamageTaken               int    `json:"totalDamageTaken"`
			TotalHeal                      int    `json:"totalHeal"`
			TotalHealsOnTeammates          int    `json:"totalHealsOnTeammates"`
			TotalMinionsKilled             int    `json:"totalMinionsKilled"`
			TotalTimeCCDealt               int    `json:"totalTimeCCDealt"`
			TotalTimeSpentDead             int    `json:"totalTimeSpentDead"`
			TotalUnitsHealed               int    `json:"totalUnitsHealed"`
			TripleKills                    int    `json:"tripleKills"`
			TrueDamageDealt                int    `json:"trueDamageDealt"`
			TrueDamageDealtToChampions     int    `json:"trueDamageDealtToChampions"`
			TrueDamageTaken                int    `json:"trueDamageTaken"`
			TurretKills                    int    `json:"turretKills"`
			TurretTakedowns                int    `json:"turretTakedowns"`
			TurretsLost                    int    `json:"turretsLost"`
			UnrealKills                    int    `json:"unrealKills"`
			VisionScore                    int    `json:"visionScore"`
			VisionWardsBoughtInGame        int    `json:"visionWardsBoughtInGame"`
			WardsKilled                    int    `json:"wardsKilled"`
			WardsPlaced                    int    `json:"wardsPlaced"`
			Win                            bool   `json:"win"`
		} `json:"participants"`
		PlatformId string `json:"platformId"`
		QueueId    int    `json:"queueId"`
		Teams      []struct {
			Bans []struct {
				ChampionId int `json:"championId"`
				PickTurn   int `json:"pickTurn"`
			} `json:"bans"`
			Objectives struct {
				Baron struct {
					First bool `json:"first"`
					Kills int  `json:"kills"`
				} `json:"baron"`
				Champion struct {
					First bool `json:"first"`
					Kills int  `json:"kills"`
				} `json:"champion"`
				Dragon struct {
					First bool `json:"first"`
					Kills int  `json:"kills"`
				} `json:"dragon"`
				Inhibitor struct {
					First bool `json:"first"`
					Kills int  `json:"kills"`
				} `json:"inhibitor"`
				RiftHerald struct {
					First bool `json:"first"`
					Kills int  `json:"kills"`
				} `json:"riftHerald"`
				Tower struct {
					First bool `json:"first"`
					Kills int  `json:"kills"`
				} `json:"tower"`
			} `json:"objectives"`
			TeamId int  `json:"teamId"`
			Win    bool `json:"win"`
		} `json:"teams"`
		TournamentCode string `json:"tournamentCode"`
	} `json:"info"`
}

func GetMatchByMatchId(matchId string) (*MatchDto, error) {
	riot.UpdateRiotApiCalls()
	resp, err := http.Get(http.GetRequest{
		Url: riot.CreateUrl(riot.RegionAsia, "/lol/match/v5/matches/"+matchId),
	})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Err
	}

	var match MatchDto
	if err := json.Unmarshal(resp.Body, &match); err != nil {
		return nil, err
	}

	return &match, nil
}
