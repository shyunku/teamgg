package service

import (
	"strconv"
	"team.gg-server/models"
	"team.gg-server/models/mixed"
	"team.gg-server/third_party/riot/api"
)

// vo_mixers manage conversion of VAO -> VO

func SummonerSummaryMixer(d models.SummonerDAO) SummonerSummaryVO {
	return SummonerSummaryVO{
		ProfileIconId: d.ProfileIconId,
		GameName:      d.GameName,
		TagLine:       d.TagLine,
		Puuid:         d.Puuid,
		SummonerLevel: d.SummonerLevel,
		LastUpdatedAt: d.LastUpdatedAt,
	}
}

func SummonerRankingMixer(d mixed.SummonerRankingMXDAO) SummonerRankingVO {
	return SummonerRankingVO{
		RatingPoints: d.RatingPoints,
		Ranking:      d.Ranking,
		Total:        d.Total,
	}
}

func SummonerRankMixer(d models.LeagueDAO) (*SummonerRankVO, error) {
	ratingPoint, err := CalculateRatingPoint(d.Tier, d.Rank, d.LeaguePoints)
	if err != nil {
		return nil, err
	}
	return &SummonerRankVO{
		Tier:        d.Tier,
		Rank:        d.Rank,
		Lp:          d.LeaguePoints,
		Wins:        d.Wins,
		Losses:      d.Losses,
		RatingPoint: ratingPoint,
	}, nil
}

func SummonerMasteryMixer(d models.MasteryDAO) SummonerMasteryVO {
	var championName *string
	champion, ok := Champions[strconv.FormatInt(d.ChampionId, 10)]
	if ok {
		championName = &champion.Name
	}

	return SummonerMasteryVO{
		ChampionId:     d.ChampionId,
		ChampionName:   championName,
		ChampionLevel:  d.ChampionLevel,
		ChampionPoints: d.ChampionPoints,
	}
}

func SummonerMatchSummaryTeamMateMixer(e mixed.MatchParticipantExtraMXDAO, summonerRankVO *SummonerRankVO, primaryPerkStyle, subPerkStyle int) TeammateVO {
	return TeammateVO{
		MatchId:                        e.MatchId,
		DataVersion:                    e.DataVersion,
		GameCreation:                   e.GameCreation,
		GameDuration:                   e.GameDuration,
		GameEndTimestamp:               e.GameEndTimestamp,
		GameId:                         e.GameId,
		GameMode:                       e.GameMode,
		GameName:                       e.GameName,
		GameStartTimestamp:             e.GameStartTimestamp,
		GameType:                       e.GameType,
		GameVersion:                    e.GameVersion,
		MapId:                          e.MapId,
		PlatformId:                     e.PlatformId,
		QueueId:                        e.QueueId,
		TournamentCode:                 e.TournamentCode,
		ParticipantId:                  e.ParticipantId,
		MatchParticipantId:             e.MatchParticipantId,
		Puuid:                          e.Puuid,
		Kills:                          e.Kills,
		Deaths:                         e.Deaths,
		Assists:                        e.Assists,
		ChampionId:                     e.ChampionId,
		ChampionLevel:                  e.ChampionLevel,
		ChampionName:                   e.ChampionName,
		ChampExperience:                e.ChampExperience,
		SummonerLevel:                  e.SummonerLevel,
		SummonerName:                   e.SummonerName,
		RiotIdName:                     e.RiotIdName,
		RiotIdTagLine:                  e.RiotIdTagLine,
		ProfileIcon:                    e.ProfileIcon,
		MagicDamageDealtToChampions:    e.MagicDamageDealtToChampions,
		PhysicalDamageDealtToChampions: e.PhysicalDamageDealtToChampions,
		TrueDamageDealtToChampions:     e.TrueDamageDealtToChampions,
		TotalDamageDealtToChampions:    e.TotalDamageDealtToChampions,
		MagicDamageTaken:               e.MagicDamageTaken,
		PhysicalDamageTaken:            e.PhysicalDamageTaken,
		TrueDamageTaken:                e.TrueDamageTaken,
		TotalDamageTaken:               e.TotalDamageTaken,
		TotalHeal:                      e.TotalHeal,
		TotalHealsOnTeammates:          e.TotalHealsOnTeammates,
		Item0:                          e.Item0,
		Item1:                          e.Item1,
		Item2:                          e.Item2,
		Item3:                          e.Item3,
		Item4:                          e.Item4,
		Item5:                          e.Item5,
		Item6:                          e.Item6,
		Spell1Casts:                    e.Spell1Casts,
		Spell2Casts:                    e.Spell2Casts,
		Spell3Casts:                    e.Spell3Casts,
		Spell4Casts:                    e.Spell4Casts,
		Summoner1Casts:                 e.Summoner1Casts,
		Summoner1Id:                    e.Summoner1Id,
		Summoner2Casts:                 e.Summoner2Casts,
		Summoner2Id:                    e.Summoner2Id,
		FirstBloodAssist:               e.FirstBloodAssist,
		FirstBloodKill:                 e.FirstBloodKill,
		DoubleKills:                    e.DoubleKills,
		TripleKills:                    e.TripleKills,
		QuadraKills:                    e.QuadraKills,
		PentaKills:                     e.PentaKills,
		TotalMinionsKilled:             e.TotalMinionsKilled,
		TotalTimeCCDealt:               e.TotalTimeCCDealt,
		NeutralMinionsKilled:           e.NeutralMinionsKilled,
		GoldSpent:                      e.GoldSpent,
		GoldEarned:                     e.GoldEarned,
		IndividualPosition:             e.IndividualPosition,
		TeamPosition:                   e.TeamPosition,
		Lane:                           e.Lane,
		Role:                           e.Role,
		TeamId:                         e.TeamId,
		VisionScore:                    e.VisionScore,
		Win:                            e.Win,
		GameEndedInEarlySurrender:      e.GameEndedInEarlySurrender,
		GameEndedInSurrender:           e.GameEndedInSurrender,
		TeamEarlySurrendered:           e.TeamEarlySurrendered,
		BaronKills:                     e.BaronKills,
		BountyLevel:                    e.BountyLevel,
		ChampionTransform:              e.ChampionTransform,
		ConsumablesPurchased:           e.ConsumablesPurchased,
		DamageDealtToBuildings:         e.DamageDealtToBuildings,
		DamageDealtToObjectives:        e.DamageDealtToObjectives,
		DamageDealtToTurrets:           e.DamageDealtToTurrets,
		DamageSelfMitigated:            e.DamageSelfMitigated,
		DetectorWardsPlaced:            e.DetectorWardsPlaced,
		DragonKills:                    e.DragonKills,
		PhysicalDamageDealt:            e.PhysicalDamageDealt,
		MagicDamageDealt:               e.MagicDamageDealt,
		TotalDamageDealt:               e.TotalDamageDealt,
		LargestCriticalStrike:          e.LargestCriticalStrike,
		LargestKillingSpree:            e.LargestKillingSpree,
		LargestMultiKill:               e.LargestMultiKill,
		FirstTowerAssist:               e.FirstTowerAssist,
		FirstTowerKill:                 e.FirstTowerKill,
		InhibitorKills:                 e.InhibitorKills,
		InhibitorTakedowns:             e.InhibitorTakedowns,
		InhibitorsLost:                 e.InhibitorsLost,
		ItemsPurchased:                 e.ItemsPurchased,
		KillingSprees:                  e.KillingSprees,
		NexusKills:                     e.NexusKills,
		NexusTakedowns:                 e.NexusTakedowns,
		NexusLost:                      e.NexusLost,
		LongestTimeSpentLiving:         e.LongestTimeSpentLiving,
		ObjectiveStolen:                e.ObjectiveStolen,
		ObjectiveStolenAssists:         e.ObjectiveStolenAssists,
		SightWardsBoughtInGame:         e.SightWardsBoughtInGame,
		VisionWardsBoughtInGame:        e.VisionWardsBoughtInGame,
		SummonerId:                     e.SummonerId,
		TimeCCingOthers:                e.TimeCCingOthers,
		TimePlayed:                     e.TimePlayed,
		TotalDamageShieldedOnTeammates: e.TotalDamageShieldedOnTeammates,
		TotalTimeSpentDead:             e.TotalTimeSpentDead,
		TotalUnitsHealed:               e.TotalUnitsHealed,
		TrueDamageDealt:                e.TrueDamageDealt,
		TurretKills:                    e.TurretKills,
		TurretTakedowns:                e.TurretTakedowns,
		TurretsLost:                    e.TurretsLost,
		UnrealKills:                    e.UnrealKills,
		WardsKilled:                    e.WardsKilled,
		WardsPlaced:                    e.WardsPlaced,
		GGScore:                        e.GetScore(),
		SummonerRank:                   summonerRankVO,
		PerkVO: PerkVO{
			PrimaryPerkStyle: primaryPerkStyle,
			SubPerkStyle:     subPerkStyle,
		},
	}
}

func SummonerMatchSummaryMixer(d models.MatchDAO, matchAvgTierRank *SummonerRankVO, myStat TeammateVO, team1 []TeammateVO, team2 []TeammateVO) MatchSummaryVO {
	return MatchSummaryVO{
		MatchId:            d.MatchId,
		GameStartTimestamp: d.GameStartTimestamp,
		GameEndTimestamp:   d.GameEndTimestamp,
		GameDuration:       d.GameDuration,
		MatchAvgTierRank:   matchAvgTierRank,
		QueueId:            d.QueueId,
		MyStat:             myStat,
		Team1:              team1,
		Team2:              team2,
	}
}

func IngameParticipantMixer(d api.SpectatorV5ParticipantDto) IngameParticipantVO {
	return IngameParticipantVO{
		ChampionId:    d.ChampionId,
		ProfileIconId: d.ProfileIconId,
		SummonerName:  d.SummonerName,
		SummonerId:    d.SummonerId,
	}
}

func CustomGameConfigurationSummaryMixer(d models.CustomGameConfigurationDAO) CustomGameConfigurationSummaryVO {
	return CustomGameConfigurationSummaryVO{
		Id:            d.Id,
		Name:          d.Name,
		LastUpdatedAt: d.LastUpdatedAt,
		Balance:       CustomGameConfigurationFairnessMixer(d),
	}
}

func CustomGameConfigurationParticipantMixer(d models.CustomGameParticipantDAO) CustomGameParticipantVO {
	return CustomGameParticipantVO{
		Position: d.Position,
		Puuid:    d.Puuid,
	}
}

func CustomGameConfigurationMixer(d models.CustomGameConfigurationDAO,
	candidates []CustomGameCandidateVO,
	team1, team2 []CustomGameParticipantVO) CustomGameConfigurationVO {
	return CustomGameConfigurationVO{
		Id:            d.Id,
		Name:          d.Name,
		CreatorUid:    d.CreatorUid,
		CreatedAt:     d.CreatedAt,
		LastUpdatedAt: d.LastUpdatedAt,
		Balance:       CustomGameConfigurationFairnessMixer(d),
		Candidates:    candidates,
		Team1:         team1,
		Team2:         team2,
		Weights:       CustomGameConfigurationWeightsMixer(d),
	}
}

func CustomGameConfigurationWeightsMixer(d models.CustomGameConfigurationDAO) CustomGameConfigurationWeightsVO {
	return CustomGameConfigurationWeightsVO{
		LineFairness:     d.LineFairnessWeight,
		TierFairness:     d.TierFairnessWeight,
		LineSatisfaction: d.LineSatisfactionWeight,
		MasteryInfluence: d.MasteryInfluenceWeight,
		TopInfluence:     d.TopInfluenceWeight,
		JungleInfluence:  d.JungleInfluenceWeight,
		MidInfluence:     d.MidInfluenceWeight,
		AdcInfluence:     d.AdcInfluenceWeight,
		SupportInfluence: d.SupportInfluenceWeight,
	}
}

func CustomGameConfigurationFairnessMixer(d models.CustomGameConfigurationDAO) CustomGameConfigurationBalanceVO {
	return CustomGameConfigurationBalanceVO{
		Fairness:         d.Fairness,
		LineFairness:     d.LineFairness,
		TierFairness:     d.TierFairness,
		LineSatisfaction: d.LineSatisfaction,
	}
}
