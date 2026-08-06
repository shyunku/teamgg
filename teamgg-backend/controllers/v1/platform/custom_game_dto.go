package platform

import "team.gg-server/service"

type GetCustomGameConfigurationsResponseDto []service.CustomGameConfigurationSummaryVO

type GetCustomGameConfigurationRequestDto struct {
	Id string `form:"id" binding:"required"`
}

type GetCustomGameConfigurationResponseDto service.CustomGameConfigurationVO

type UpdateCustomGameConfigurationNameRequestDto struct {
	Id   string `json:"id" binding:"required"`
	Name string `json:"name" binding:"required"`
}

type GetTierRankRequestDto struct {
	RatingPoint *float64 `form:"ratingPoint" binding:"required"`
}

type GetTierRankResponseDto struct {
	Tier string `json:"tier"`
	Rank string `json:"rank"`
	Lp   int64  `json:"lp"`
}

type GetCustomConfigurationBalanceRequestDto struct {
	Id string `form:"id" binding:"required"`
}

type GetCustomConfigurationSummaryResponseDto service.CustomGameConfigurationSummaryVO

type AddCandidateToCustomGameRequestDto struct {
	CustomGameConfigId string `json:"customGameConfigId" binding:"required"`
	Name               string `json:"name" binding:"required"`
	TagLine            string `json:"tagLine" binding:"required"`
}

type AddCandidateToCustomGameResponseDto service.CustomGameCandidateVO

type DeleteCandidateFromCustomGameRequestDto struct {
	CustomGameConfigId string `form:"customGameConfigId" binding:"required"`
	Puuid              string `form:"puuid" binding:"required"`
}

type ArrangeCustomGameParticipantRequestDto struct {
	CustomGameConfigId string `json:"customGameConfigId" binding:"required"`
	Puuid              string `json:"puuid" binding:"required"`
	Team               int    `json:"team" binding:"required"`
	TargetPosition     string `json:"targetPosition" binding:"required"`
}

type UnarrangeCustomGameParticipantRequestDto struct {
	CustomGameConfigId string `json:"customGameConfigId" binding:"required"`
	Puuid              string `json:"puuid" binding:"required"`
}

type SetCustomGameParticipantFavorPositionRequestDto struct {
	CustomGameConfigId string `json:"customGameConfigId" binding:"required"`
	Puuid              string `json:"puuid" binding:"required"`
	FavorPosition      string `json:"favorPosition" binding:"required"`
	Strength           *int   `json:"strength" binding:"required"`
}

type SetCustomGameParticipantLineMasteryRequestDto struct {
	CustomGameConfigId string `json:"customGameConfigId" binding:"required"`
	Puuid              string `json:"puuid" binding:"required"`
	Position           string `json:"position" binding:"required"`
	Level              *int   `json:"level" binding:"required"`
}

type SaveCustomGameDefaultFavorPositionRequestDto struct {
	CustomGameConfigId string `json:"customGameConfigId" binding:"required"`
	Puuid              string `json:"puuid" binding:"required"`
}

type SetCustomGameCandidateCustomTierRankRequestDto struct {
	CustomGameConfigId string  `json:"customGameConfigId" binding:"required"`
	Puuid              string  `json:"puuid" binding:"required"`
	Tier               *string `json:"tier"`
	Rank               *string `json:"rank"`
}

type SetCustomGameCandidateCustomColorLabelRequestDto struct {
	CustomGameConfigId string `json:"customGameConfigId" binding:"required"`
	Puuid              string `json:"puuid" binding:"required"`
	ColorCode          *int   `json:"colorCode" binding:"required,number,gte=0,lte=5"`
}

type DeleteCustomGameCandidateCustomColorLabelRequestDto struct {
	CustomGameConfigId string `form:"customGameConfigId" binding:"required"`
}

type OptimizeCustomGameConfigurationRequestDto struct {
	Id                     string   `json:"id" binding:"required"`
	LineFairnessWeight     *float64 `json:"lineFairnessWeight" binding:"required"`
	TierFairnessWeight     *float64 `json:"tierFairnessWeight" binding:"required"`
	LineSatisfactionWeight *float64 `json:"lineSatisfactionWeight" binding:"required"`
	MasteryInfluenceWeight *float64 `json:"masteryInfluenceWeight" binding:"required"`
}

type UtilityRequestDto struct {
	Id string `json:"id" binding:"required"`
}
