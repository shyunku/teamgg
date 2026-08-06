package platform

import "team.gg-server/models"

type CreateReplayAnalysisRequestDto struct {
	CustomGameConfigId string `json:"customGameId" binding:"required"`
	FileName           string `json:"fileName" binding:"required"`
	FileSize           int64  `json:"fileSize" binding:"required,gt=0"`
}

type CreateReplayAnalysisResponseDto struct {
	Analysis     models.CustomGameReplayAnalysisDAO `json:"analysis"`
	UploadUrl    string                             `json:"uploadUrl"`
	UploadTicket string                             `json:"uploadTicket"`
}

type ListReplayAnalysesRequestDto struct {
	CustomGameConfigId string `form:"customGameId" binding:"required"`
}
