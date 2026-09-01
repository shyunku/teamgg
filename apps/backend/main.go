package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	log "github.com/shyunku-libraries/go-logger"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"team.gg-server/controllers"
	"team.gg-server/core"
	"team.gg-server/libs/crypto"
	"team.gg-server/libs/db"
	"team.gg-server/migrations"
	"team.gg-server/models"
	"team.gg-server/service"
	"team.gg-server/service/statistics"
	"team.gg-server/third_party/riot"
	"team.gg-server/util"
	"time"
)

func main() {
	fmt.Println(`
	████████╗███████╗ █████╗ ███╗   ███╗    ██████╗  ██████╗ 
	╚══██╔══╝██╔════╝██╔══██╗████╗ ████║   ██╔════╝ ██╔════╝ 
	   ██║   █████╗  ███████║██╔████╔██║   ██║  ███╗██║  ███╗
	   ██║   ██╔══╝  ██╔══██║██║╚██╔╝██║   ██║   ██║██║   ██║
	   ██║   ███████╗██║  ██║██║ ╚═╝ ██║██╗╚██████╔╝╚██████╔╝
	   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝╚═╝ ╚═════╝  ╚═════╝ 
	`)
	log.Info("team.gg Server is now starting...")
	log.Info("Version: ", core.Version)

	// randomize seed
	rand.Seed(time.Now().UnixNano())

	// Create Cancel Context
	ctx, cancel := context.WithCancel(context.Background())
	var waitGroup sync.WaitGroup

	// Load environment variables
	log.Info("Initializing environments...")
	environmentPath, pathErr := filepath.Abs(".env")
	if pathErr != nil {
		environmentPath = ".env"
	}
	log.Infof("Looking for optional local environment file: %s", environmentPath)
	if err := godotenv.Load(environmentPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Info("Optional local environment file not found; continuing with process environment variables (expected with Docker Compose env_file)")
		} else {
			log.Error(err)
			os.Exit(-1)
		}
	}

	migrationOnly := len(os.Args) > 1 && strings.EqualFold(os.Args[1], "migrate")
	championDetailCollectionOnly := len(os.Args) > 1 && strings.EqualFold(os.Args[1], "collect-champion-detail")
	numericKeyBackfillOnly := len(os.Args) > 1 && strings.EqualFold(os.Args[1], "backfill-numeric-keys")
	masteryNumericShadowOnly := len(os.Args) > 1 && strings.EqualFold(os.Args[1], "prepare-mastery-numeric-shadow")
	requiredEnvironment := []string{
		"DB_USER", "DB_PASSWORD", "DB_HOST", "DB_PORT", "DB_NAME",
	}
	if !migrationOnly && !numericKeyBackfillOnly && !masteryNumericShadowOnly {
		requiredEnvironment = append(requiredEnvironment,
			"APP_SERVER_PORT",
			"JWT_ACCESS_SECRET",
			"JWT_ACCESS_EXPIRE",
			"JWT_REFRESH_SECRET",
			"JWT_REFRESH_EXPIRE",
			"RSO_CLIENT_ID",
			"RSO_CLIENT_SECRET",
			"RSO_CLIENT_CALLBACK_URI",
			"DEBUG",
			"IS_PROD",
			"REPLAY_ANALYZER_BASE_URL",
			"REPLAY_ANALYZER_SHARED_SECRET",
		)
	}
	if err := util.CheckEnvironmentVariables(requiredEnvironment); err != nil {
		log.Error(err)
		os.Exit(-1)
	}

	// Init Root database
	var err error
	log.Info("Initializing database...")
	if db.Root, err = db.Initiate(nil); err != nil {
		log.Error(err)
		os.Exit(-4)
	}
	migrationMode, err := migrations.ResolveMode(migrationOnly)
	if err != nil {
		log.Error(err)
		os.Exit(-4)
	}
	log.Infof("Database migration mode: %s", migrationMode)
	if err = migrations.Run(ctx, db.Root.DB, migrationMode, core.Version); err != nil {
		log.Error(err)
		os.Exit(-4)
	}
	if migrationOnly {
		log.Info("Database migrations completed")
		if err := db.Root.Close(); err != nil {
			log.Error(err)
			os.Exit(-4)
		}
		return
	}
	if numericKeyBackfillOnly {
		options := migrations.NumericKeyBackfillOptionsFromEnvironment()
		log.Infof(
			"Numeric key backfill starting: batchSize=%d workLimit=%s",
			options.BatchSize, options.WorkLimit,
		)
		result, backfillErr := migrations.BackfillNumericKeys(ctx, db.Root.DB, options)
		if backfillErr != nil {
			log.Error(backfillErr)
			os.Exit(-4)
		}
		log.Infof(
			"Numeric key backfill finished: ready=%t summoners=%d/%t matches=%d/%t participants=%d/%t children=%d/%t masteriesReady=%t",
			result.Ready,
			result.SummonersProcessed, result.SummonersCompleted,
			result.MatchesProcessed, result.MatchesCompleted,
			result.ParticipantsProcessed, result.ParticipantsCompleted,
			result.ChildrenProcessed, result.ChildrenCompleted,
			result.MasteriesReady,
		)
		if err := db.Root.Close(); err != nil {
			log.Error(err)
			os.Exit(-4)
		}
		return
	}
	if masteryNumericShadowOnly {
		options := migrations.MasteryNumericShadowOptionsFromEnvironment()
		log.Infof(
			"Mastery numeric shadow starting: batchSize=%d workLimit=%s maxBatches=%d offlineAcknowledged=%t disableBinlog=%t",
			options.BatchSize, options.WorkLimit, options.MaxBatches, options.OfflineAcknowledged, options.DisableBinlog,
		)
		result, shadowErr := migrations.PrepareMasteryNumericShadow(ctx, db.Root.DB, options)
		if shadowErr != nil {
			log.Error(shadowErr)
			os.Exit(-4)
		}
		log.Infof(
			"Mastery numeric shadow finished: processed=%d total=%d copied=%t validated=%t",
			result.ProcessedThisRun, result.ProcessedTotal, result.CopyCompleted, result.Validated,
		)
		if err := db.Root.Close(); err != nil {
			log.Error(err)
			os.Exit(-4)
		}
		return
	}
	if err := models.ConfigureMasteryReadSource(os.Getenv("MASTERY_READ_SOURCE")); err != nil {
		log.Error(err)
		os.Exit(-4)
	}
	if models.MasteryNumericV2ReadsEnabled() {
		if err := migrations.ValidateMasteryNumericShadowCutover(ctx, db.Root.DB); err != nil {
			log.Error(fmt.Errorf("mastery numeric read cutover is not ready: %w", err))
			os.Exit(-4)
		}
		log.Info("Mastery read source: numeric_v2")
	} else {
		log.Info("Mastery read source: legacy")
	}
	if err := service.RootDatabaseInitializer(db.Root.DB); err != nil {
		log.Error(fmt.Errorf("failed to initialize database: %w", err))
		os.Exit(-4)
	}
	if statistics.StatisticsDB, err = db.Initiate(nil); err != nil {
		log.Error(err)
		os.Exit(-5)
	}
	// Each statistics repository holds at most one dedicated connection while
	// collecting. Keep this analytical pool bounded instead of inheriting the
	// general-purpose 100-connection default.
	statistics.StatisticsDB.SetMaxIdleConns(3)
	statistics.StatisticsDB.SetMaxOpenConns(4)

	// preload
	if err := core.Preload(); err != nil {
		log.Error(err)
		os.Exit(-2)
	}

	// preload service
	if err := service.Preload(); err != nil {
		log.Error(err)
		os.Exit(-3)
	}

	// print debug state
	if core.IsProduction {
		log.Info("Running in production mode...")
	} else {
		log.Info("Running in development mode...")
	}
	if core.DebugMode {
		log.Info("Debug diagnostics are enabled...")
	}

	// Initialize statistics repositories before optional background services so
	// the one-shot maintenance command cannot start DataExplorer workers.
	log.Info("Initializing statistics repository...")
	if err := statistics.InitializeStatisticRepos(); err != nil {
		log.Error(err)
		os.Exit(-6)
	}
	if championDetailCollectionOnly {
		log.Info("Running one-shot champion detail statistics collection...")
		if err := statistics.ChampionDetailStatisticsRepo.CollectUntilReady(ctx); err != nil {
			log.Error(err)
			os.Exit(-6)
		}
		log.Info("One-shot champion detail statistics collection completed")
		if err := statistics.StatisticsDB.Finalize(); err != nil {
			log.Error(err)
			os.Exit(-5)
		}
		if err := db.Root.Finalize(); err != nil {
			log.Error(err)
			os.Exit(-4)
		}
		return
	}

	// Init in-memory database
	log.Info("Initializing in-memory database...")
	db.InMemoryDB = db.NewRedis()

	// Init 3rd party services
	log.Info("Initializing 3rd party services...")
	riot.Init()

	// Init jwt secret key
	log.Info("Initializing jwt secret key...")
	crypto.Initialize()

	// randomize seed
	rand.Seed(time.Now().UnixNano())

	// Start data explorer
	log.Info("Starting data explorer...")
	de := service.NewDataExplorer()
	go de.Loop()

	// start statistics repository loop
	log.Info("Starting statistics repository loops...")
	startStatisticsLoop := func(loop func(context.Context)) {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			loop(ctx)
		}()
	}
	startStatisticsLoop(statistics.ChampionDetailStatisticsRepo.Loop)
	startStatisticsLoop(statistics.TierStatisticsRepo.Loop)
	startStatisticsLoop(statistics.MasteryStatisticsRepo.Loop)

	// Run web server with gin
	waitGroup.Add(1)
	go controllers.RunGin(ctx, &waitGroup)

	// prepare finalize
	service.PrepareFinalize(cancel, &waitGroup, []service.Finalizer{
		func() error {
			if err := db.Root.Finalize(); err != nil {
				log.Fatal(err)
				return err
			}
			if err := statistics.StatisticsDB.Finalize(); err != nil {
				log.Fatal(err)
				return err
			}
			return nil
		},
	})
}
