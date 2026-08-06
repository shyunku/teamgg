package api

import (
	"encoding/json"
	"team.gg-server/libs/http"
	"team.gg-server/third_party/riot"
)

type MasteryItemDto struct {
	Puuid                        string `json:"puuid"`
	ChampionPointsUntilNextLevel int64  `json:"championPointsUntilNextLevel"`
	ChestGranted                 bool   `json:"chestGranted"`
	ChampionId                   int64  `json:"championId"`
	LastPlayTime                 int64  `json:"lastPlayTime"`
	ChampionLevel                int    `json:"championLevel"`
	SummonerId                   string `json:"summonerId"`
	ChampionPoints               int    `json:"championPoints"`
	ChampionPointsSinceLastLevel int64  `json:"championPointsSinceLastLevel"`
	TokensEarned                 int    `json:"tokensEarned"`
}

type MasteryDto []MasteryItemDto

func GetMasteryByPuuid(puuid string) (*MasteryDto, error) {
	riot.UpdateRiotApiCalls()
	resp, err := http.Get(http.GetRequest{
		Url: riot.CreateUrl(riot.RegionKr, "/lol/champion-mastery/v4/champion-masteries/by-puuid/"+puuid),
	})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Err
	}

	var mastery MasteryDto
	if err := json.Unmarshal(resp.Body, &mastery); err != nil {
		return nil, err
	}

	return &mastery, nil
}
