package service

import (
	"fmt"
	log "github.com/shyunku-libraries/go-logger"
	"team.gg-server/core"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/models/mixed"
	"team.gg-server/types"
	"team.gg-server/util"
)

// vo_getters configure vo with VAO and mixed-VAO (null-safe)

func GetSummonerSummaryVO_byPuuid(puuid string) (*SummonerSummaryVO, error) {
	if core.DebugOnProd {
		defer util.InspectFunctionExecutionTime()()
	}

	// find summoner by name on db
	summonerDao, exists, err := models.GetSummonerDAO_byPuuid(db.Root, puuid)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("summoner dao not found with puuid (%s)", puuid)
	}
	summonerVo := SummonerSummaryMixer(*summonerDao)
	return &summonerVo, nil
}

func GetSummonerExtraVO(puuid string, soloRank *SummonerRankVO) (*SummonerExtraVO, error) {
	if core.DebugOnProd {
		defer util.InspectFunctionExecutionTime()()
	}

	rankingVO, err := getSummonerRankingVO(puuid)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	matchParticipantExtraMXDAOs, err := mixed.GetMatchParticipantExtraMXDAOs_byQueueId(puuid, types.QueueTypeAll, 30)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	ggScoreSum := 0.0
	validGGScores := 0
	ggScoreAvg := 0.0
	for _, extraMXDAO := range matchParticipantExtraMXDAOs {
		if extraMXDAO.GameEndedInEarlySurrender {
			continue
		}
		ggScoreSum += extraMXDAO.GetScore()
		validGGScores++
	}
	if validGGScores > 0 {
		ggScoreAvg = ggScoreSum / float64(validGGScores)
	}

	// TODO :: add some extra fun things (statistics_models: tags)

	var predictedRankVO *SummonerRankVO
	predictedMMR := 0.0
	if ggScoreAvg > 0 && soloRank != nil {
		predictedMMR = PredictMatchMakingRating(float64(soloRank.RatingPoint), ggScoreAvg)
		reverseCalculatedRP := MMRtoRatingPoint(predictedMMR)
		tier, rank, lp, err := CalculateTierRank(reverseCalculatedRP)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		predictedRankVO = &SummonerRankVO{
			Tier:        string(tier),
			Rank:        string(rank),
			Lp:          int(lp),
			RatingPoint: int64(reverseCalculatedRP),
			Wins:        0,
			Losses:      0,
		}
	}

	return &SummonerExtraVO{
		Ranking:          *rankingVO,
		RecentAvgGGScore: ggScoreAvg,
		PredictedMMR:     predictedMMR,
		PredictedRank:    predictedRankVO,
	}, nil
}

func getSummonerRankingVO(puuid string) (*SummonerRankingVO, error) {
	summonerRankingMXDAO, err := mixed.GetSummonerSoloRankingMXDAO(db.Root, puuid)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	summonerVo := SummonerRankingMixer(*summonerRankingMXDAO)
	return &summonerVo, nil
}

// GetSummonerRankVO returns SummonerRankVO by puuid and rankType
// this function assumes that summoner info & rank info has consistency
func GetSummonerRankVO(puuid string, rankType string) (*SummonerRankVO, error) {
	if core.DebugOnProd {
		defer util.InspectFunctionExecutionTime()()
	}
	if core.DebugOnProd {
		defer util.InspectFunctionExecutionTime()()
	}
	leagueDAO, exists, err := models.GetLeagueDAO(db.Root, puuid, rankType)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	leagueVo, err := SummonerRankMixer(*leagueDAO)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return leagueVo, nil
}

func GetSummonerMasteryVOs(puuid string) ([]SummonerMasteryVO, error) {
	if core.DebugOnProd {
		defer util.InspectFunctionExecutionTime()()
	}
	masteries, err := models.GetMasteryDAOs(db.Root, puuid)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	var masteryVos []SummonerMasteryVO
	for _, mastery := range masteries {
		masteryVos = append(masteryVos, SummonerMasteryMixer(*mastery))
	}
	return masteryVos, nil
}

// GetSummonerRecentMatchSummaryVOs_byQueueId 특정 플레이어의 최근 특정 큐 (ex. 솔랭, 자랭) 의 최근 매치 요약 정보를 가져옵니다.
func GetSummonerRecentMatchSummaryVOs_byQueueId(puuid string, queueId, count int) ([]MatchSummaryVO, error) {
	if core.DebugOnProd {
		defer util.InspectFunctionExecutionTime()()
	}
	var matchDAOs []models.MatchDAO
	var err error
	if queueId == types.QueueTypeAll {
		matchDAOs, err = models.GetMatchDAOs_byPuuid(db.Root, puuid, count)
	} else {
		matchDAOs, err = models.GetMatchDAOs_byQueueId(db.Root, puuid, queueId, count)
	}
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return getSummonerMatchSummaryVOs(puuid, matchDAOs)
}

// GetSummonerMatchSummaryVOs_byQueueId_before 특정 시간 이전의 매치 요약 정보를 가져옵니다.
func GetSummonerMatchSummaryVOs_byQueueId_before(puuid string, queueId int, before int64, count int) ([]MatchSummaryVO, error) {
	var matchDAOs []models.MatchDAO
	var err error
	if queueId == types.QueueTypeAll {
		matchDAOs, err = models.GetMatchDAOs_byPuuid_before(db.Root, puuid, before, count)
	} else {
		matchDAOs, err = models.GetMatchDAOs_byQueueId_before(db.Root, puuid, queueId, before, count)
	}
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return getSummonerMatchSummaryVOs(puuid, matchDAOs)
}

func getSummonerMatchSummaryVOs(puuid string, matchDAOs []models.MatchDAO) ([]MatchSummaryVO, error) {
	if core.DebugOnProd {
		defer util.InspectFunctionExecutionTime()()
	}

	promise := util.NewPromise[models.MatchDAO, MatchSummaryVO]()
	getMatchSummary := func(resolve chan<- MatchSummaryVO, reject chan<- error, matchDAO models.MatchDAO) {
		matchExtraMXDAOs, err := mixed.GetMatchParticipantExtraMXDAOs_byMatchId(matchDAO.MatchId)
		if err != nil {
			log.Error(err)
			reject <- err
		}

		var myStat *TeammateVO
		ratingPoint := 0.0
		validRankers := 0
		team1Participants := make([]TeammateVO, 0)
		team2Participants := make([]TeammateVO, 0)
		for _, matchExtraDAO := range matchExtraMXDAOs {
			var summonerRankVO *SummonerRankVO
			queueType := types.RankTypeSolo
			if matchDAO.QueueId == types.QueueTypeFlex {
				queueType = types.RankTypeFlex
			}

			summonerRankVO, err = GetSummonerRankVO(matchExtraDAO.Puuid, queueType)
			if err != nil {
				log.Error(err)
				reject <- err
			}
			if summonerRankVO != nil {
				ratingPoint += float64(summonerRankVO.RatingPoint)
				validRankers++
			}

			perks, err := models.GetMatchParticipantPerkStyleDAOs(db.Root, matchExtraDAO.MatchParticipantId)
			if err != nil {
				log.Warn(err)
				reject <- err
			}
			primaryPerkStyle := 0
			subPerkStyle := 0
			for _, perk := range perks {
				if perk.Description == types.PerkStyleDescriptionTypePrimary {
					primaryPerkStyle = perk.Style
				} else if perk.Description == types.PerkStyleDescriptionTypeSub {
					subPerkStyle = perk.Style
				}
			}
			teamMate := SummonerMatchSummaryTeamMateMixer(matchExtraDAO, summonerRankVO, primaryPerkStyle, subPerkStyle)
			if matchExtraDAO.TeamId == 100 {
				team1Participants = append(team1Participants, teamMate)
			} else {
				team2Participants = append(team2Participants, teamMate)
			}

			if matchExtraDAO.Puuid == puuid {
				myStat = &teamMate
			}
		}
		if myStat == nil {
			reject <- fmt.Errorf("myStat is nil")
		}

		var avgMatchRankVO *SummonerRankVO
		if validRankers > 0 {
			avgMatchRatingPoint := ratingPoint / float64(validRankers)
			tier, rank, lp, err := CalculateTierRank(avgMatchRatingPoint)
			if err != nil {
				log.Error(err)
				reject <- err
			}

			avgMatchRankVO = &SummonerRankVO{
				Tier:        string(tier),
				Rank:        string(rank),
				Lp:          int(lp),
				RatingPoint: int64(int(avgMatchRatingPoint)),
			}
		}

		resolve <- SummonerMatchSummaryMixer(
			matchDAO,
			avgMatchRankVO,
			*myStat,
			team1Participants,
			team2Participants,
		)
	}

	for _, matchDAO := range matchDAOs {
		promise.Add(getMatchSummary, matchDAO)
	}

	matchSummaryVOs := make([]MatchSummaryVO, 0)
	for _, result := range promise.All() {
		if result.Err != nil {
			log.Error(result.Err)
			return nil, result.Err
		}
		matchSummaryVOs = append(matchSummaryVOs, *result.Result)
	}

	return matchSummaryVOs, nil
}

func GetCustomGameConfigurationVOs(uid string) ([]CustomGameConfigurationSummaryVO, error) {
	customGameConfigurationDAOs, err := models.GetCustomGameDAOs_byCreatorUid(db.Root, uid)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	customGameConfigurationVOs := make([]CustomGameConfigurationSummaryVO, 0)
	for _, customGameConfigurationDAO := range customGameConfigurationDAOs {
		customGameConfigurationVOs = append(customGameConfigurationVOs, CustomGameConfigurationSummaryMixer(customGameConfigurationDAO))
	}

	return customGameConfigurationVOs, nil
}

func GetCustomGameCandidateVO(candidateDAO models.CustomGameCandidateDAO) (*CustomGameCandidateVO, error) {
	summonerDao, exists, err := models.GetSummonerDAO_byPuuid(db.Root, candidateDAO.Puuid)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("summoner dao not found with puuid (%s)", candidateDAO.Puuid)
	}
	summonerVO := SummonerSummaryMixer(*summonerDao)

	var soloLeagueVO *SummonerRankVO
	soloLeagueDAO, exists, err := models.GetLeagueDAO(db.Root, candidateDAO.Puuid, types.RankTypeSolo)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if exists {
		soloLeagueVO, err = SummonerRankMixer(*soloLeagueDAO)
		if err != nil {
			log.Error(err)
			return nil, err
		}
	}

	var flexLeagueVO *SummonerRankVO
	flexLeagueDAO, exists, err := models.GetLeagueDAO(db.Root, candidateDAO.Puuid, types.RankTypeFlex)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if exists {
		flexLeagueVO, err = SummonerRankMixer(*flexLeagueDAO)
		if err != nil {
			log.Error(err)
			return nil, err
		}
	}

	var customRankVO *SummonerRankVO
	if candidateDAO.CustomTier != nil && candidateDAO.CustomRank != nil {
		customRankVO = &SummonerRankVO{
			Tier:   *candidateDAO.CustomTier,
			Rank:   *candidateDAO.CustomRank,
			Lp:     0,
			Wins:   0,
			Losses: 0,
		}
		ratingPoint, err := CalculateRatingPoint(*candidateDAO.CustomTier, *candidateDAO.CustomRank, 0)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		customRankVO.RatingPoint = ratingPoint
	}

	positionFavorVO := CustomGameCandidatePositionFavorVO{
		Top:     candidateDAO.FlavorTop,
		Jungle:  candidateDAO.FlavorJungle,
		Mid:     candidateDAO.FlavorMid,
		Adc:     candidateDAO.FlavorAdc,
		Support: candidateDAO.FlavorSupport,
	}

	masteryDAO, err := models.GetMasteryDAOs(db.Root, candidateDAO.Puuid)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	masteryVOs := make([]SummonerMasteryVO, 0)
	for _, mastery := range masteryDAO {
		masteryVOs = append(masteryVOs, SummonerMasteryMixer(*mastery))
	}

	return &CustomGameCandidateVO{
		Summary:       summonerVO,
		SoloRank:      soloLeagueVO,
		FlexRank:      flexLeagueVO,
		CustomRank:    customRankVO,
		PositionFavor: positionFavorVO,
		Mastery:       masteryVOs,
	}, nil
}

func GetCustomGameConfigurationVO(configurationId string) (*CustomGameConfigurationVO, error) {
	customGameConfigurationDAO, exists, err := models.GetCustomGameDAO_byId(db.Root, configurationId)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("custom game configuration dao not found with id (%s)", configurationId)
	}

	candidateVOs := make([]CustomGameCandidateVO, 0)
	team1ParticipantsVOs := make([]CustomGameParticipantVO, 0)
	team2ParticipantsVOs := make([]CustomGameParticipantVO, 0)

	// get participant color code
	participantColorLabelDAOs, err := models.GetCustomGameParticipantColorLabelDAOs_byCustomGameConfigId(db.Root, configurationId)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	colorLabels := make(map[string]int)
	for _, participantColorLabelDAO := range participantColorLabelDAOs {
		colorLabels[participantColorLabelDAO.Puuid] = participantColorLabelDAO.ColorCode
	}

	// get candidates
	candidateDAOs, err := models.GetCustomGameCandidateDAOs_byCustomGameConfigId(db.Root, configurationId)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	for _, candidateDAO := range candidateDAOs {
		colorCode, exists := colorLabels[candidateDAO.Puuid]
		if !exists {
			colorCode = 0
		}

		summonerVO, err := GetCustomGameCandidateVO(candidateDAO)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		candidateVOs = append(candidateVOs, CustomGameCandidateVO{
			Summary:       summonerVO.Summary,
			SoloRank:      summonerVO.SoloRank,
			FlexRank:      summonerVO.FlexRank,
			CustomRank:    summonerVO.CustomRank,
			PositionFavor: summonerVO.PositionFavor,
			Mastery:       summonerVO.Mastery,
			ColorCode:     colorCode,
		})
	}

	// get participants
	participantDAOs, err := models.GetCustomGameParticipantDAOs_byCustomGameConfigId(db.Root, configurationId)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	for _, participantDAO := range participantDAOs {
		participantVO := CustomGameConfigurationParticipantMixer(*participantDAO)
		if participantDAO.Team == 1 {
			team1ParticipantsVOs = append(team1ParticipantsVOs, participantVO)
		} else {
			team2ParticipantsVOs = append(team2ParticipantsVOs, participantVO)
		}
	}

	customGameConfigurationVO := CustomGameConfigurationMixer(
		*customGameConfigurationDAO,
		candidateVOs,
		team1ParticipantsVOs,
		team2ParticipantsVOs,
	)
	return &customGameConfigurationVO, nil
}

func GetCustomGameConfigurationBalanceVO(customGameConfigId string) (*CustomGameConfigurationBalanceVO, error) {
	customGameConfigDAO, exists, err := models.GetCustomGameDAO_byId(db.Root, customGameConfigId)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("custom game config dao not found with id (%s)", customGameConfigId)
	}

	fairnessVO := CustomGameConfigurationFairnessMixer(*customGameConfigDAO)
	return &fairnessVO, nil
}
