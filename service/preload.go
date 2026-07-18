package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/jmoiron/sqlx"
	"github.com/schollz/progressbar/v3"
	log "github.com/shyunku-libraries/go-logger"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"team.gg-server/core"
	"team.gg-server/libs/db"
	"team.gg-server/libs/http"
	"team.gg-server/models"
	"team.gg-server/util"
)

// https://raw.communitydragon.org/latest/plugins/rcp-be-lol-game-data/global/ko_kr/v1/

var (
	DataDragonVersion      = ""
	LocalDataDragonVersion = ""

	Champions      = make(map[string]ChampionDataVO)      // key: champion key
	SummonerSpells = make(map[string]SummonerSpellDataVO) // key: summoner spell key
	Items          = make(map[int]ItemDataVO)             // key: item id
	Perks          = make(map[int]PerkInfoVO)             // key: perk id
	PerkStyles     = make(map[int]PerkStyleInfoVO)        // key: perk style id

	RootDatabaseInitializer = func(db *sqlx.DB) error {
		if err := models.EnsureUserIdentityTable(db); err != nil {
			return err
		}

		// find static data tables
		var staticTierRankTable interface{}

		if err := db.Get(&staticTierRankTable, "SELECT 1 FROM information_schema.tables WHERE table_name = 'static_tier_ranks' LIMIT 1"); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// create static tier ranks table
				if _, err := db.Exec(`
					CREATE TABLE static_tier_ranks (
					    id INT AUTO_INCREMENT PRIMARY KEY,
						tier_label VARCHAR(20) NOT NULL,
						rank_label VARCHAR(20) NOT NULL,
						score INT NOT NULL,
						UNIQUE KEY (tier_label, rank_label),
						UNIQUE KEY (score),
						INDEX (tier_label),
						INDEX (rank_label)
					)
				`); err != nil {
					log.Error(err)
					return err
				}
			} else {
				log.Error(err)
				return err
			}
		}

		tx, err := db.BeginTxx(context.Background(), nil)
		if err != nil {
			log.Error(err)
			return err
		}

		tierKeys := make([]Tier, 0)
		for tier, _ := range TierRankMap {
			if tier == TierUnranked {
				continue
			}
			tierKeys = append(tierKeys, tier)
		}

		sort.SliceStable(tierKeys, func(i, j int) bool {
			tierLevelI, err := GetTierLevel(tierKeys[i])
			if err != nil {
				log.Fatal(err)
				_ = tx.Rollback()
				os.Exit(-2)
			}
			tierLevelJ, err := GetTierLevel(tierKeys[j])
			if err != nil {
				log.Fatal(err)
				_ = tx.Rollback()
				os.Exit(-3)
			}
			return tierLevelI < tierLevelJ
		})
		for _, tier := range tierKeys {
			originalRanks, ok := TierRankMap[tier]
			if !ok {
				log.Fatal(fmt.Errorf("tier not found: %s", tier))
				_ = tx.Rollback()
				os.Exit(-4)
			}

			ranks := make([]Rank, len(originalRanks))
			copy(ranks, originalRanks)

			sort.SliceStable(ranks, func(i, j int) bool {
				rankLevelI, err := GetRankLevel(tier, ranks[i])
				if err != nil {
					log.Fatal(err)
					_ = tx.Rollback()
					os.Exit(-4)
				}
				rankLevelJ, err := GetRankLevel(tier, ranks[j])
				if err != nil {
					log.Fatal(err)
					_ = tx.Rollback()
					os.Exit(-5)
				}
				return rankLevelI < rankLevelJ
			})
			for _, rank := range ranks {
				lp := 0
				if tier == TierGrandmaster {
					lp = TierHighGrandmasterUnderBound
				} else if tier == TierChallenger {
					lp = TierHighChallengerUnderBound
				}
				ratingPoint, err := CalculateRatingPoint(string(tier), string(rank), lp)
				if err != nil {
					log.Error(err)
					_ = tx.Rollback()
					return err
				}

				// get current score
				needUpdate := false
				var currentScore int
				if err := tx.Get(&currentScore, "SELECT score FROM static_tier_ranks WHERE tier_label = ? AND rank_label = ?", tier, rank); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						needUpdate = true
						currentScore = -1
					} else {
						log.Error(err)
						_ = tx.Rollback()
						return err
					}
				}

				if needUpdate {
					// upsert
					if _, err := tx.Exec(`
						INSERT INTO static_tier_ranks (tier_label, rank_label, score) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE score = ?
					`, tier, rank, ratingPoint, ratingPoint); err != nil {
						log.Error(err)
						_ = tx.Rollback()
						return err
					}
				}
			}
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		return nil
	}
)

func Preload() error {
	log.Debugf("Service preload started...")

	// load data dragon version
	var err error
	DataDragonVersion, err = GetLatestDataDragonVersion()
	if err != nil {
		return err
	}
	log.Debugf("DataDragon version: %s", DataDragonVersion)

	// manage & load data dragon files
	if err := SanitizeAndLoadDataDragonFile(); err != nil {
		return err
	}

	// load summoner spell data
	if err := LoadSummonerSpellsData(); err != nil {
		return err
	}

	// load items data
	if err := LoadItemsData(); err != nil {
		return err
	}

	// save items data to db
	if err := SaveItemsDataToDB(); err != nil {
		return err
	}

	// load champion data
	championsInfo, err := GetChampionData()
	if err != nil {
		return err
	}
	for _, champion := range championsInfo.Data {
		Champions[champion.Key] = champion
	}
	log.Debugf("%d Champion data loaded", len(Champions))

	// load perks data
	perksData, err := GetCDragonPerksData()
	if err != nil {
		return err
	}
	for _, perk := range perksData {
		Perks[perk.Id] = perk
	}

	// load perk styles data
	perkStylesData, err := GetCDragonPerkStylesData()
	if err != nil {
		return err
	}
	for _, perkStyle := range (*perkStylesData).Styles {
		PerkStyles[perkStyle.Id] = perkStyle
	}

	return nil
}

func SanitizeAndLoadDataDragonFile() error {
	// check if latest data dragon is downloaded
	projectRoot := util.GetProjectRootDirectory()

	// create tmp download dir if not exists
	tmpDownloadDirPath := fmt.Sprintf("%s/datafiles/tmp", projectRoot)
	err := os.MkdirAll(tmpDownloadDirPath, os.ModePerm)
	if err != nil {
		log.Error(err)
		return err
	}

	// create data dragon download dir if not exists
	dataDragonDirPath := fmt.Sprintf("%s/datafiles/data_dragon", projectRoot)
	err = os.MkdirAll(dataDragonDirPath, os.ModePerm)
	if err != nil {
		log.Error(err)
		return err
	}

	destDataDragonPath := fmt.Sprintf("%s/%s", dataDragonDirPath, DataDragonVersion)

	targetDataDragonDirPath := fmt.Sprintf("%s/%s", dataDragonDirPath, DataDragonVersion)
	targetTarGzPath := fmt.Sprintf("%s/dragontail-%s.tgz", tmpDownloadDirPath, DataDragonVersion)
	if _, err := os.Stat(targetDataDragonDirPath); errors.Is(err, os.ErrNotExist) {
		log.Infof("latest data dragon not found (%s)", DataDragonVersion)

		// check if tar.gz file exists
		if _, err := os.Stat(targetTarGzPath); errors.Is(err, os.ErrNotExist) {
			log.Infof("latest data dragon tar.gz file not found, downloading...")

			// tar.gz file not exists, download latest data dragon
			url := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/dragontail-%s.tgz", DataDragonVersion)
			resp, err := http.StreamGet(http.GetRequest{
				Url: url,
			})
			if err != nil {
				log.Error(err)
				return err
			}
			defer resp.Stream.Close()

			// save data dragon tar.gz file
			out, err := os.Create(targetTarGzPath)
			if err != nil {
				return err
			}
			defer out.Close()

			// progress bar
			progressBar := progressbar.DefaultBytes(
				resp.ContentLength,
				"Downloading data dragon",
			)

			// copy data dragon
			_, err = io.Copy(io.MultiWriter(out, progressBar), resp.Stream)
			if err != nil {
				return err
			}
		}

		// extract data dragon
		log.Infof("extracting data dragon tar.gz file (%s)...", targetTarGzPath)
		if err := util.UnTarGz(targetTarGzPath, destDataDragonPath); err != nil {
			return err
		}

		log.Infof("Data dragon extraction done! dest: %s", destDataDragonPath)
	} else {
		log.Infof("Data dragon is already latest: %s", DataDragonVersion)
	}

	// remove ddragon folders in dir (except latest)
	files, err := os.ReadDir(dataDragonDirPath)
	if err != nil {
		return err
	}
	var ddragonEntries []os.DirEntry
	var latestDdragonEntry os.DirEntry
	for _, file := range files {
		ddragonEntries = append(ddragonEntries, file)
		entryName := file.Name()

		if latestDdragonEntry == nil {
			latestDdragonEntry = file
		} else {
			latestVersion, err := version.NewVersion(latestDdragonEntry.Name())
			if err != nil {
				log.Warnf("failed to parse version (%s)", latestDdragonEntry.Name())
				continue
			}
			currentVersion, err := version.NewVersion(entryName)
			if err != nil {
				log.Warnf("failed to parse version (%s)", entryName)
				continue
			}

			if currentVersion.GreaterThan(latestVersion) {
				log.Debugf("version %s > %s", currentVersion, latestVersion)
				latestDdragonEntry = file
			}
		}
	}
	if latestDdragonEntry != nil {
		log.Infof("latest data dragon version: %s, keep alive", latestDdragonEntry.Name())
		LocalDataDragonVersion = latestDdragonEntry.Name()
		for _, file := range ddragonEntries {
			if file == latestDdragonEntry {
				continue
			}
			removingDirPath := fmt.Sprintf("%s/%s", dataDragonDirPath, file.Name())
			log.Debugf("removing old data dragon dir (%s)...", removingDirPath)
			if err := os.RemoveAll(removingDirPath); err != nil {
				log.Warnf("failed to remove old data dragon dir (%s)", file.Name())
			}
			log.Debugf("remove %s complete", file.Name())
		}
		if len(ddragonEntries) > 1 {
			log.Debugf("%d old data dragon files removed", len(ddragonEntries)-1)
		}
	} else {
		if core.UrgentMode {
			log.Warnf("latest data dragon not found")
		} else {
			return errors.New("latest data dragon not found")
		}
	}

	// remove old data dragon if exists (except latest)
	// get all data dragon tar files
	files, err = os.ReadDir(tmpDownloadDirPath)
	if err != nil {
		return err
	}

	// remove old data dragon tar.gz files
	var ddragonTarGzEntries []os.DirEntry
	var latestDdragonTarGzEntry os.DirEntry
	var latestDdragonTarGzVersion string
	for _, file := range files {
		ddragonTarGzEntries = append(ddragonTarGzEntries, file)
		entryName := file.Name()

		versionRegex := regexp.MustCompile(`\d+\.\d+\.\d+`)
		entryVersion := versionRegex.FindString(entryName)
		if entryVersion == "" {
			log.Warnf("failed to parse version (%s)", entryName)
			continue
		}

		updateLatest := func() {
			latestDdragonTarGzEntry = file
			latestDdragonTarGzVersion = entryVersion
		}

		if latestDdragonTarGzEntry == nil || latestDdragonTarGzVersion == "" {
			updateLatest()
		} else {
			latestVersion, err := version.NewVersion(latestDdragonTarGzVersion)
			if err != nil {
				log.Warnf("failed to parse version (%s)", latestDdragonTarGzVersion)
				continue
			}
			currentVersion, err := version.NewVersion(entryVersion)
			if err != nil {
				log.Warnf("failed to parse version (%s)", entryName)
				continue
			}

			if currentVersion.GreaterThan(latestVersion) {
				updateLatest()
			}
		}
	}
	if latestDdragonTarGzEntry != nil {
		removed := 1
		log.Infof("latest data dragon tar.gz version: %s, keep alive", latestDdragonTarGzEntry.Name())
		for _, file := range ddragonTarGzEntries {
			if file == latestDdragonTarGzEntry {
				continue
			}
			if err := os.RemoveAll(fmt.Sprintf("%s/%s", tmpDownloadDirPath, file.Name())); err != nil {
				log.Warnf("failed to remove old data dragon tar.gz (%s)", file.Name())
			} else {
				removed++
			}
		}
		if removed > 0 {
			log.Debugf("%d old data dragon tar.gz removed", removed)
		}
	}

	return nil
}

func LoadSummonerSpellsData() error {
	// load summoner spells
	var SummonerSpellsDto DDragonSummonerJsonDto
	if err := LoadDDragonKorFile(&SummonerSpellsDto, "/summoner.json"); err != nil {
		return err
	}

	// save to memory
	SummonerSpells = map[string]SummonerSpellDataVO{}
	for _, summonerSpell := range SummonerSpellsDto.Data {
		SummonerSpells[summonerSpell.Key] = summonerSpell
	}

	log.Debugf("%d summoner spells loaded", len(SummonerSpells))
	return nil
}

func LoadItemsData() error {
	// load items
	var ItemsDto DDragonItemJsonDto
	if err := LoadDDragonKorFile(&ItemsDto, "/item.json"); err != nil {
		return err
	}

	// save to memory
	Items = map[int]ItemDataVO{}
	for id, item := range ItemsDto.Data {
		itemId, err := strconv.Atoi(id)
		if err != nil {
			return err
		}
		Items[itemId] = item
	}

	log.Debugf("%d items loaded", len(Items))
	return nil
}

func SaveItemsDataToDB() error {
	// upsert db: items, item_tags
	// find static data tables
	var (
		itemsTable    interface{}
		itemTagsTable interface{}
	)

	tx, err := db.Root.BeginTxx(context.Background(), nil)
	if err != nil {
		log.Error(err)
		return err
	}

	if err := tx.Get(&itemsTable, "SELECT 1 FROM information_schema.tables WHERE table_name = 'static_items' LIMIT 1"); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// create static tier ranks table
			if _, err := tx.Exec(`
				CREATE TABLE static_items (
					id INT PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					description TEXT NOT NULL,
					plaintext TEXT NOT NULL,
					required_ally VARCHAR(255) NULL,
					depth INT NULL,
					gold_base INT NOT NULL,
					gold_purchasable TINYINT NOT NULL,
					gold_total INT NOT NULL,
					gold_sell INT NOT NULL,
					INDEX (name),
					INDEX (depth DESC),
					INDEX (gold_total DESC)
				)
			`); err != nil {
				log.Error(err)
				_ = tx.Rollback()
				return err
			}
		} else {
			log.Error(err)
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Get(&itemTagsTable, "SELECT 1 FROM information_schema.tables WHERE table_name = 'static_item_tags' LIMIT 1"); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// create static tier ranks table
			if _, err := tx.Exec(`
				CREATE TABLE static_item_tags (
					item_id INT NOT NULL,
					tag VARCHAR(100) NOT NULL,
					PRIMARY KEY (item_id, tag),
					FOREIGN KEY (item_id) REFERENCES static_items(id),
					INDEX (tag)
				)
			`); err != nil {
				log.Error(err)
				_ = tx.Rollback()
				return err
			}
		} else {
			log.Error(err)
			_ = tx.Rollback()
			return err
		}
	}

	// upsert items
	for id, item := range Items {
		if _, err := tx.Exec(`
			INSERT INTO static_items (id, name, description, plaintext, required_ally, depth, gold_base, gold_purchasable, gold_total, gold_sell) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE name = ?, description = ?, plaintext = ?, required_ally = ?, depth = ?,
			                        gold_base = ?, gold_purchasable = ?, gold_total = ?, gold_sell = ?
		`, id, item.Name, item.Description, item.Plaintext, item.RequiredAlly, item.Depth, item.Gold.Base, item.Gold.Purchasable, item.Gold.Total, item.Gold.Sell,
			item.Name, item.Description, item.Plaintext, item.RequiredAlly, item.Depth, item.Gold.Base, item.Gold.Purchasable, item.Gold.Total, item.Gold.Sell,
		); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			return err
		}

		// upsert item tags
		for _, tag := range item.Tags {
			if _, err := tx.Exec(`
				INSERT INTO static_item_tags (item_id, tag) VALUES (?, ?) ON DUPLICATE KEY UPDATE tag = VALUES(tag)
			`, id, tag); err != nil {
				log.Error(err)
				_ = tx.Rollback()
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func GetChampionData() (*ChampionsInfoDto, error) {
	resp, err := http.Get(http.GetRequest{
		Url: fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/data/ko_KR/champion.json", DataDragonVersion),
	})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Err
	}

	var champions ChampionsInfoDto
	if err := json.Unmarshal(resp.Body, &champions); err != nil {
		return nil, err
	}

	return &champions, nil
}

func GetLatestDataDragonVersion() (string, error) {
	resp, err := http.Get(http.GetRequest{
		Url: "https://ddragon.leagueoflegends.com/api/versions.json",
	})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", resp.Err
	}

	var versions []string
	if err := json.Unmarshal(resp.Body, &versions); err != nil {
		return "", err
	}

	return versions[0], nil
}

func GetCDragonPerksData() (PerksInfoDto, error) {
	path := "https://raw.communitydragon.org/latest/plugins/rcp-be-lol-game-data/global/ko_kr/v1/perks.json"
	resp, err := http.Get(http.GetRequest{
		Url: path,
	})
	if err != nil {
		return nil, err
	}

	var perksData PerksInfoDto
	if err := json.Unmarshal(resp.Body, &perksData); err != nil {
		return nil, err
	}

	return perksData, nil
}

func GetCDragonPerkStylesData() (*PerkStylesInfoDto, error) {
	path := "https://raw.communitydragon.org/latest/plugins/rcp-be-lol-game-data/global/ko_kr/v1/perkstyles.json"
	resp, err := http.Get(http.GetRequest{
		Url: path,
	})
	if err != nil {
		return nil, err
	}

	var perkStylesData PerkStylesInfoDto
	if err := json.Unmarshal(resp.Body, &perkStylesData); err != nil {
		return nil, err
	}

	return &perkStylesData, nil
}
