package api

import (
	"encoding/json"
	"team.gg-server/libs/http"
	"team.gg-server/third_party/riot"
)

type AccountByRiotIdDto struct {
	Puuid    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

func GetAccountByRiotId(gameName, tagLine string) (*AccountByRiotIdDto, int, error) {
	riot.UpdateRiotApiCalls()
	resp, err := http.Get(http.GetRequest{
		Url: riot.CreateUrl(riot.RegionAsia, "/riot/account/v1/accounts/by-riot-id/"+riot.Encode(gameName)+"/"+riot.Encode(tagLine)),
	})
	if err != nil {
		return nil, 0, err
	}
	if !resp.Success {
		return nil, resp.StatusCode, resp.Err
	}

	var account AccountByRiotIdDto
	if err := json.Unmarshal(resp.Body, &account); err != nil {
		return nil, resp.StatusCode, err
	}

	return &account, resp.StatusCode, nil
}

func GetAccountByPuuid(puuid string) (*AccountByRiotIdDto, int, error) {
	riot.UpdateRiotApiCalls()
	resp, err := http.Get(http.GetRequest{
		Url: riot.CreateUrl(riot.RegionAsia, "/riot/account/v1/accounts/by-puuid/"+puuid),
	})
	if err != nil {
		return nil, 0, err
	}
	if !resp.Success {
		return nil, resp.StatusCode, resp.Err
	}

	var account AccountByRiotIdDto
	if err := json.Unmarshal(resp.Body, &account); err != nil {
		return nil, resp.StatusCode, err
	}

	return &account, resp.StatusCode, nil
}
