package service

import (
	"strconv"
	"time"
)

/* ------------------------ Service VOs ------------------------ */

type SummonerSummaryVO struct {
	ProfileIconId int       `json:"profileIconId"`
	GameName      string    `json:"gameName"`
	TagLine       string    `json:"tagLine"`
	Puuid         string    `json:"puuid"`
	SummonerLevel int64     `json:"summonerLevel"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}

type SummonerRankingVO struct {
	RatingPoints int `json:"ratingPoints"`
	Ranking      int `json:"ranking"`
	Total        int `json:"total"`
}

type SummonerRankVO struct {
	Tier        string `json:"tier"`
	Rank        string `json:"rank"`
	Lp          int    `json:"lp"`
	Wins        int    `json:"wins"`
	Losses      int    `json:"losses"`
	RatingPoint int64  `json:"ratingPoint"`
}

type SummonerMasteryVO struct {
	ChampionId     int64   `json:"championId"`
	ChampionName   *string `json:"championName"`
	ChampionLevel  int     `json:"championLevel"`
	ChampionPoints int     `json:"championPoints"`
}

type SummonerExtraVO struct {
	Ranking          SummonerRankingVO `json:"ranking"`
	RecentAvgGGScore float64           `json:"recentAvgGGScore"`
	PredictedMMR     float64           `json:"predictedMMR"`
	PredictedRank    *SummonerRankVO   `json:"predictedRank"`
}

type PerkVO struct {
	PrimaryPerkStyle int `json:"primaryPerkStyle"`
	SubPerkStyle     int `json:"subPerkStyle"`
}

type TeammateVO struct {
	// match
	MatchId            string `json:"matchId"`
	DataVersion        string `json:"dataVersion"`
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
	PlatformId         string `json:"platformId"`
	QueueId            int    `json:"queueId"`
	TournamentCode     string `json:"tournamentCode"`

	// participant
	ParticipantId                  int    `json:"participantId"`
	MatchParticipantId             string `json:"matchParticipantId"`
	Puuid                          string `json:"puuid"`
	Kills                          int    `json:"kills"`
	Deaths                         int    `json:"deaths"`
	Assists                        int    `json:"assists"`
	ChampionId                     int    `json:"championId"`
	ChampionLevel                  int    `json:"championLevel"`
	ChampionName                   string `json:"championName"`
	ChampExperience                int    `json:"champExperience"`
	SummonerLevel                  int    `json:"summonerLevel"`
	SummonerName                   string `json:"summonerName"`
	RiotIdName                     string `json:"riotIdName"`
	RiotIdTagLine                  string `json:"riotIdTagLine"`
	ProfileIcon                    int    `json:"profileIcon"`
	MagicDamageDealtToChampions    int    `json:"magicDamageDealtToChampions"`
	PhysicalDamageDealtToChampions int    `json:"physicalDamageDealtToChampions"`
	TrueDamageDealtToChampions     int    `json:"trueDamageDealtToChampions"`
	TotalDamageDealtToChampions    int    `json:"totalDamageDealtToChampions"`
	MagicDamageTaken               int    `json:"magicDamageTaken"`
	PhysicalDamageTaken            int    `json:"physicalDamageTaken"`
	TrueDamageTaken                int    `json:"trueDamageTaken"`
	TotalDamageTaken               int    `json:"totalDamageTaken"`
	TotalHeal                      int    `json:"totalHeal"`
	TotalHealsOnTeammates          int    `json:"totalHealsOnTeammates"`
	Item0                          int    `json:"item0"`
	Item1                          int    `json:"item1"`
	Item2                          int    `json:"item2"`
	Item3                          int    `json:"item3"`
	Item4                          int    `json:"item4"`
	Item5                          int    `json:"item5"`
	Item6                          int    `json:"item6"`
	Spell1Casts                    int    `json:"spell1Casts"`
	Spell2Casts                    int    `json:"spell2Casts"`
	Spell3Casts                    int    `json:"spell3Casts"`
	Spell4Casts                    int    `json:"spell4Casts"`
	Summoner1Casts                 int    `json:"summoner1Casts"`
	Summoner1Id                    int    `json:"summoner1Id"`
	Summoner2Casts                 int    `json:"summoner2Casts"`
	Summoner2Id                    int    `json:"summoner2Id"`
	FirstBloodAssist               bool   `json:"firstBloodAssist"`
	FirstBloodKill                 bool   `json:"firstBloodKill"`
	DoubleKills                    int    `json:"doubleKills"`
	TripleKills                    int    `json:"tripleKills"`
	QuadraKills                    int    `json:"quadraKills"`
	PentaKills                     int    `json:"pentaKills"`
	TotalMinionsKilled             int    `json:"totalMinionsKilled"`
	TotalTimeCCDealt               int    `json:"totalTimeCCDealt"`
	NeutralMinionsKilled           int    `json:"neutralMinionsKilled"`
	GoldSpent                      int    `json:"goldSpent"`
	GoldEarned                     int    `json:"goldEarned"`
	IndividualPosition             string `json:"individualPosition"`
	TeamPosition                   string `json:"teamPosition"`
	Lane                           string `json:"lane"`
	Role                           string `json:"role"`
	TeamId                         int    `json:"teamId"`
	VisionScore                    int    `json:"visionScore"`
	Win                            bool   `json:"win"`
	GameEndedInEarlySurrender      bool   `json:"gameEndedInEarlySurrender"`
	GameEndedInSurrender           bool   `json:"gameEndedInSurrender"`
	TeamEarlySurrendered           bool   `json:"teamEarlySurrendered"`

	//	Details
	BaronKills                     int    `json:"baronKills"`
	BountyLevel                    int    `json:"bountyLevel"`
	ChampionTransform              int    `json:"championTransform"`
	ConsumablesPurchased           int    `json:"consumablesPurchased"`
	DamageDealtToBuildings         int    `json:"damageDealtToBuildings"`  // 건물에 입힌 피해량
	DamageDealtToObjectives        int    `json:"damageDealtToObjectives"` // 목표물에 입힌 피해량
	DamageDealtToTurrets           int    `json:"damageDealtToTurrets"`    // 포탑에 입힌 피해량
	DamageSelfMitigated            int    `json:"damageSelfMitigated"`     // 자신에 대한 피해 감소량
	DetectorWardsPlaced            int    `json:"detectorWardsPlaced"`
	DragonKills                    int    `json:"dragonKills"`
	PhysicalDamageDealt            int    `json:"physicalDamageDealt"`
	MagicDamageDealt               int    `json:"magicDamageDealt"`
	TotalDamageDealt               int    `json:"totalDamageDealt"`
	LargestCriticalStrike          int    `json:"largestCriticalStrike"`
	LargestKillingSpree            int    `json:"largestKillingSpree"`
	LargestMultiKill               int    `json:"largestMultiKill"`
	FirstTowerAssist               bool   `json:"firstTowerAssist"`
	FirstTowerKill                 bool   `json:"firstTowerKill"`
	InhibitorKills                 int    `json:"inhibitorKills"`
	InhibitorTakedowns             int    `json:"inhibitorTakedowns"`
	InhibitorsLost                 int    `json:"inhibitorsLost"`
	ItemsPurchased                 int    `json:"itemsPurchased"`
	KillingSprees                  int    `json:"killingSprees"`
	NexusKills                     int    `json:"nexusKills"`
	NexusTakedowns                 int    `json:"nexusTakedowns"`
	NexusLost                      int    `json:"nexusLost"`
	LongestTimeSpentLiving         int    `json:"longestTimeSpentLiving"`
	ObjectiveStolen                int    `json:"objectiveStolen"`
	ObjectiveStolenAssists         int    `json:"objectiveStolenAssists"`
	SightWardsBoughtInGame         int    `json:"sightWardsBoughtInGame"`
	VisionWardsBoughtInGame        int    `json:"visionWardsBoughtInGame"`
	SummonerId                     string `json:"summonerId"`
	TimeCCingOthers                int    `json:"timeCCingOthers"`
	TimePlayed                     int    `json:"timePlayed"`
	TotalDamageShieldedOnTeammates int    `json:"totalDamageShieldedOnTeammates"`
	TotalTimeSpentDead             int    `json:"totalTimeSpentDead"`
	TotalUnitsHealed               int    `json:"totalUnitsHealed"`
	TrueDamageDealt                int    `json:"trueDamageDealt"`
	TurretKills                    int    `json:"turretKills"`
	TurretTakedowns                int    `json:"turretTakedowns"`
	TurretsLost                    int    `json:"turretsLost"`
	UnrealKills                    int    `json:"unrealKills"`
	WardsKilled                    int    `json:"wardsKilled"`
	WardsPlaced                    int    `json:"wardsPlaced"`

	// additional
	GGScore      float64         `json:"ggScore"`
	SummonerRank *SummonerRankVO `json:"summonerRank"`
	PerkVO
}

type MatchSummaryVO struct {
	MatchId            string          `json:"matchId"`
	GameStartTimestamp int64           `json:"gameStartTimestamp"`
	GameEndTimestamp   int64           `json:"gameEndTimestamp"`
	GameDuration       int64           `json:"gameDuration"`
	MatchAvgTierRank   *SummonerRankVO `json:"matchAvgTierRank"`
	QueueId            int             `json:"queueId"`
	MyStat             TeammateVO      `json:"myStat"`
	Team1              []TeammateVO    `json:"team1"`
	Team2              []TeammateVO    `json:"team2"`
}

type IngameParticipantVO struct {
	ChampionId    int64  `json:"championId"`
	ProfileIconId int64  `json:"profileIconId"`
	SummonerName  string `json:"summonerName"`
	SummonerId    string `json:"summonerId"`
}

type ChampionStatisticVO struct {
	ChampionId   int    `json:"championId"`
	ChampionName string `json:"championName"`

	Win   int `json:"win"`
	Total int `json:"total"`

	AvgPickRate float64 `json:"avgPickRate"`
	AvgBanRate  float64 `json:"avgBanRate"`

	AvgMinionsKilled float64 `json:"avgMinionsKilled"`
	AvgKills         float64 `json:"avgKills"`
	AvgDeaths        float64 `json:"avgDeaths"`
	AvgAssists       float64 `json:"avgAssists"`
	AvgGoldEarned    float64 `json:"avgGoldEarned"`
}

/* ------------------------ Preload VOs ------------------------ */

type ChampionDataVO struct {
	Version string `json:"version"`
	Id      string `json:"id"` // 챔피언 아이디 (ex. Aatrox)
	Key     string `json:"key"`
	Name    string `json:"name"`
	Title   string `json:"title"` // 챔피언 컨셉 (ex. 아트록스: 다르킨의 검)
	Blurb   string `json:"blurb"` // 챔피언 스토리
	Info    struct {
		Attack     int `json:"attack"`
		Defense    int `json:"defense"`
		Magic      int `json:"magic"`
		Difficulty int `json:"difficulty"`
	} `json:"info"`
	Image struct {
		Full   string `json:"full"`
		Sprite string `json:"sprite"`
		Group  string `json:"group"`
		X      int    `json:"x"`
		Y      int    `json:"y"`
		W      int    `json:"w"`
		H      int    `json:"h"`
	} `json:"image"`
	Tags    []string `json:"tags"`
	Partype string   `json:"partype"`
	Stats   struct {
		Hp                   float64 `json:"hp"`
		Hpperlevel           float64 `json:"hpperlevel"`
		Mp                   float64 `json:"mp"`
		Mpperlevel           float64 `json:"mpperlevel"`
		Movespeed            float64 `json:"movespeed"`
		Armor                float64 `json:"armor"`
		Armorperlevel        float64 `json:"armorperlevel"`
		Spellblock           float64 `json:"spellblock"`
		Spellblockperlevel   float64 `json:"spellblockperlevel"`
		Attackrange          float64 `json:"attackrange"`
		Hpregen              float64 `json:"hpregen"`
		Hpregenperlevel      float64 `json:"hpregenperlevel"`
		Mpregen              float64 `json:"mpregen"`
		Mpregenperlevel      float64 `json:"mpregenperlevel"`
		Crit                 float64 `json:"crit"`
		Critperlevel         float64 `json:"critperlevel"`
		Attackdamage         float64 `json:"attackdamage"`
		Attackdamageperlevel float64 `json:"attackdamageperlevel"`
		Attackspeedperlevel  float64 `json:"attackspeedperlevel"`
		Attackspeed          float64 `json:"attackspeed"`
	} `json:"stats"`
}

type SummonerSpellDataVO struct {
	Id            string        `json:"id,omitempty"`
	Name          string        `json:"name,omitempty"`
	Description   string        `json:"description,omitempty"`
	Tooltip       string        `json:"tooltip,omitempty"`
	Maxrank       int           `json:"maxrank,omitempty"`
	Cooldown      []float64     `json:"cooldown,omitempty"`
	CooldownBurn  string        `json:"cooldownBurn,omitempty"`
	Cost          []int         `json:"cost,omitempty"`
	CostBurn      string        `json:"costBurn,omitempty"`
	Datavalues    interface{}   `json:"datavalues,omitempty"`
	Effect        []interface{} `json:"effect,omitempty"`
	EffectBurn    []*string     `json:"effectBurn,omitempty"`
	Vars          []interface{} `json:"vars,omitempty"`
	Key           string        `json:"key,omitempty"`
	SummonerLevel int           `json:"summonerLevel,omitempty"`
	Modes         []string      `json:"modes,omitempty"`
	CostType      string        `json:"costType,omitempty"`
	Maxammo       string        `json:"maxammo,omitempty"`
	Range         []int         `json:"range,omitempty"`
	RangeBurn     string        `json:"rangeBurn,omitempty"`
	Image         struct {
		Full   string `json:"full,omitempty"`
		Sprite string `json:"sprite,omitempty"`
		Group  string `json:"group,omitempty"`
		X      int    `json:"x,omitempty"`
		Y      int    `json:"y,omitempty"`
		W      int    `json:"w,omitempty"`
		H      int    `json:"h,omitempty"`
	} `json:"image"`
	Resource string `json:"resource,omitempty"`
}

type ItemDataVO struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Colloq       string    `json:"colloq"`
	Plaintext    string    `json:"plaintext"`
	Into         *[]string `json:"into"`
	From         *[]string `json:"from"`
	RequiredAlly *string   `json:"requiredAlly"`
	Image        struct {
		Full   string `json:"full"`
		Sprite string `json:"sprite"`
		Group  string `json:"group"`
		X      int    `json:"x"`
		Y      int    `json:"y"`
		W      int    `json:"w"`
		H      int    `json:"h"`
	} `json:"image"`
	Gold struct {
		Base        int  `json:"base"`
		Purchasable bool `json:"purchasable"`
		Total       int  `json:"total"`
		Sell        int  `json:"sell"`
	} `json:"gold"`
	Tags  []string               `json:"tags"`
	Maps  map[string]bool        `json:"maps"`
	Stats map[string]interface{} `json:"stats"`
	Depth *int                   `json:"depth"`
}

func (idv *ItemDataVO) AvailableOnMapId(mapId int) bool {
	available, exists := idv.Maps[strconv.Itoa(mapId)]
	return exists && available
}

type PerkInfoVO struct {
	Id                                  int         `json:"id"`
	Name                                string      `json:"name"`
	MajorChangePatchVersion             string      `json:"majorChangePatchVersion"`
	ToolTip                             string      `json:"tooltip"`
	ShortDesc                           string      `json:"shortDesc"`
	LongDesc                            string      `json:"longDesc"`
	RecommendationDescriptor            string      `json:"recommendationDescriptor"`
	IconPath                            string      `json:"iconPath"`
	EndOfGameStatDescs                  []string    `json:"endOfGameStatDescs"`
	RecommendationDescriptionAttributes interface{} `json:"recommendationDescriptionAttributes"`
}

type PerkStyleInfoVO struct {
	Id               int         `json:"id"`
	Name             string      `json:"name"`
	ToolTip          string      `json:"tooltip"`
	IconPath         string      `json:"iconPath"`
	AssetMap         interface{} `json:"assetMap"`
	IsAdvanced       bool        `json:"isAdvanced"`
	AllowedSubStyles []int       `json:"allowedSubStyles"`
	SubStyleBonus    []struct {
		StyleId int `json:"styleId"`
		PerkId  int `json:"perkId"`
	} `json:"subStyleBonus"`
	Slots []struct {
		Type      string `json:"type"`
		SlotLabel string `json:"slotLabel"`
		Perks     []int  `json:"perks"`
	} `json:"slots"`
	DefaultPageName            string `json:"defaultPageName"`
	DefaultSubStyle            int    `json:"defaultSubStyle"`
	DefaultPerks               []int  `json:"defaultPerks"`
	DefaultPerksWhenSplashed   []int  `json:"defaultPerksWhenSplashed"`
	DefaultStatModsPerSubStyle []struct {
		Id    string `json:"id"`
		Perks []int  `json:"perks"`
	} `json:"defaultStatModsPerSubStyle"`
}

type CustomGameConfigurationSummaryVO struct {
	Id            string                           `json:"id"`
	Name          string                           `json:"name"`
	LastUpdatedAt time.Time                        `json:"lastUpdatedAt"`
	IsOptimizing  bool                             `json:"isOptimizing"`
	Balance       CustomGameConfigurationBalanceVO `json:"balance"`
}

type CustomGameCandidatePositionFavorVO struct {
	Top     int `json:"top"`
	Jungle  int `json:"jungle"`
	Mid     int `json:"mid"`
	Adc     int `json:"adc"`
	Support int `json:"support"`
}

type CustomGameCandidateVO struct {
	Summary       SummonerSummaryVO                  `json:"summary"`
	SoloRank      *SummonerRankVO                    `json:"soloRank"`
	FlexRank      *SummonerRankVO                    `json:"flexRank"`
	CustomRank    *SummonerRankVO                    `json:"customRank"`
	PositionFavor CustomGameCandidatePositionFavorVO `json:"positionFavor"`
	Mastery       []SummonerMasteryVO                `json:"mastery"`
	ColorCode     int                                `json:"colorCode"`
}

func (c *CustomGameCandidateVO) GetRepresentativeRank() *SummonerRankVO {
	if c.CustomRank != nil {
		return c.CustomRank
	} else if c.SoloRank != nil {
		return c.SoloRank
	} else {
		return c.FlexRank
	}
}

func (c *CustomGameCandidateVO) GetRepresentativeRatingPoint() int64 {
	representativeRank := c.GetRepresentativeRank()
	if representativeRank == nil {
		return 0
	}
	return representativeRank.RatingPoint
}

type CustomGameTeamParticipantVO struct {
	CustomGameCandidateVO
	Team     int    `json:"team"`
	Position string `json:"position"`
}

type CustomGameParticipantVO struct {
	Position string `json:"position"`
	Puuid    string `json:"puuid"`
}

type CustomGameConfigurationBalanceVO struct {
	Fairness         float64 `json:"fairness"`
	LineFairness     float64 `json:"lineFairness"`
	TierFairness     float64 `json:"tierFairness"`
	LineSatisfaction float64 `json:"lineSatisfaction"`
}

type CustomGameConfigurationWeightsVO struct {
	LineFairness     float64 `json:"lineFairness"`
	TierFairness     float64 `json:"tierFairness"`
	LineSatisfaction float64 `json:"lineSatisfaction"`

	TopInfluence     float64 `json:"topInfluence"`
	JungleInfluence  float64 `json:"jungleInfluence"`
	MidInfluence     float64 `json:"midInfluence"`
	AdcInfluence     float64 `json:"adcInfluence"`
	SupportInfluence float64 `json:"supportInfluence"`
}

type CustomGameTeamPositionVO struct {
	Team     int
	Position string
}

type CustomGameConfigurationVO struct {
	Id            string                           `json:"id"`
	Name          string                           `json:"name"`
	CreatorUid    string                           `json:"creatorUid"`
	CreatedAt     time.Time                        `json:"createdAt"`
	LastUpdatedAt time.Time                        `json:"lastUpdatedAt"`
	IsOptimizing  bool                             `json:"isOptimizing"`
	CanManage     bool                             `json:"canManage"`
	OwnedPuuids   []string                         `json:"ownedPuuids"`
	Balance       CustomGameConfigurationBalanceVO `json:"balance"`

	Weights CustomGameConfigurationWeightsVO `json:"weights"`

	Candidates []CustomGameCandidateVO `json:"candidates"`

	Team1 []CustomGameParticipantVO `json:"team1"`
	Team2 []CustomGameParticipantVO `json:"team2"`
}
