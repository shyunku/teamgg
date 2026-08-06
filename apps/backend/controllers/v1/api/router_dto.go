package api

type GetSummonerPuuidRequestDto struct {
	GameName string  `form:"gameName" binding:"required"`
	TagLine  *string `form:"tagLine" binding:"required"`
}

type SetSummonerLineFavorRequestDto struct {
	UserId    string `json:"userId" binding:"required"`
	Strengths []int  `json:"strengths" binding:"required"`
}

type GetDiscordIntegrationsRequestDto struct {
	Token string `form:"token" binding:"required"`
}
