package api

import (
	"encoding/json"
	"team.gg-server/libs/http"
	"team.gg-server/third_party/riot"
)

type SpectatorParticipantDto struct {
	ChampionId int64 `json:"championId"`
	Perks      struct {
		PerkIds      []int64 `json:"perkIds"`
		PerkStyle    int64   `json:"perkStyle"`
		PerkSubStyle int64   `json:"perkSubStyle"`
	} `json:"perks"`
	ProfileIconId            int64  `json:"profileIconId"`
	Bot                      bool   `json:"bot"`
	TeamId                   int64  `json:"teamId"`
	SummonerName             string `json:"summonerName"`
	SummonerId               string `json:"summonerId"`
	Spell1Id                 int64  `json:"spell1Id"`
	Spell2Id                 int64  `json:"spell2Id"`
	GameCustomizationObjects []struct {
		Category string `json:"category"`
		Content  string `json:"content"`
	} `json:"gameCustomizationObjects"`
}

type SpectatorDto struct {
	GameId          int64  `json:"gameId"`
	GameType        string `json:"gameType"`
	GameStartTime   int64  `json:"gameStartTime"`
	MapId           int64  `json:"mapId"`
	GameLength      int64  `json:"gameLength"`
	PlatformId      string `json:"platformId"`
	GameMode        string `json:"gameMode"`
	BannedChampions []struct {
		PickTurn   int   `json:"pickTurn"`
		ChampionId int64 `json:"championId"`
		TeamId     int64 `json:"teamId"`
	} `json:"bannedChampions"`
	GameQueueConfigId int64 `json:"gameQueueConfigId"`
	Observers         struct {
		EncryptionKey string `json:"encryptionKey"`
	} `json:"observers"`
	Participants []SpectatorParticipantDto `json:"participants"`
}

func GetSpectatorInfo(summonerId string) (*SpectatorDto, int, error) {
	riot.UpdateRiotApiCalls()
	resp, err := http.Get(http.GetRequest{
		Url: riot.CreateUrl(riot.RegionKr, "/lol/spectator/v4/active-games/by-summoner/"+summonerId),
	})
	if err != nil {
		return nil, 0, err
	}
	if !resp.Success {
		return nil, resp.StatusCode, resp.Err
	}

	var spectator SpectatorDto
	if err := json.Unmarshal(resp.Body, &spectator); err != nil {
		return nil, resp.StatusCode, err
	}

	return &spectator, resp.StatusCode, nil
}
