package api

import (
	"encoding/json"
	"team.gg-server/libs/http"
	"team.gg-server/third_party/riot"
)

type SummonerDto struct {
	ProfileIconId int    `json:"profileIconId"`
	RevisionDate  int64  `json:"revisionDate"`
	Puuid         string `json:"puuid"`
	SummonerLevel int64  `json:"summonerLevel"`
}

func GetSummonerByName(name string) (*SummonerDto, int, error) {
	riot.UpdateRiotApiCalls()
	resp, err := http.Get(http.GetRequest{
		Url: riot.CreateUrl(riot.RegionKr, "/lol/summoner/v4/summoners/by-name/"+name),
	})
	if err != nil {
		return nil, 0, err
	}
	if !resp.Success {
		return nil, resp.StatusCode, resp.Err
	}

	var summoner SummonerDto
	if err := json.Unmarshal(resp.Body, &summoner); err != nil {
		return nil, resp.StatusCode, err
	}
	return &summoner, resp.StatusCode, nil
}

func GetSummonerByPuuid(puuid string) (*SummonerDto, int, error) {
	riot.UpdateRiotApiCalls()
	resp, err := http.Get(http.GetRequest{
		Url: riot.CreateUrl(riot.RegionKr, "/lol/summoner/v4/summoners/by-puuid/"+puuid),
	})
	if err != nil {
		return nil, 0, err
	}
	if !resp.Success {
		return nil, resp.StatusCode, resp.Err
	}

	var summoner SummonerDto
	if err := json.Unmarshal(resp.Body, &summoner); err != nil {
		return nil, resp.StatusCode, err
	}

	return &summoner, resp.StatusCode, nil
}
