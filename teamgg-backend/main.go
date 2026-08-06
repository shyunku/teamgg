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
	"sync"
	"team.gg-server/controllers"
	"team.gg-server/core"
	"team.gg-server/libs/crypto"
	"team.gg-server/libs/db"
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
	log.Infof("Loading environment file: %s", environmentPath)
	if err := godotenv.Load(environmentPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Info("Environment file not found; using process environment variables")
		} else {
			log.Error(err)
			os.Exit(-1)
		}
	}

	// Check environment variables
	if err := util.CheckEnvironmentVariables([]string{
		"APP_SERVER_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_HOST",
		"DB_PORT",
		"DB_NAME",
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
	}); err != nil {
		log.Error(err)
		os.Exit(-1)
	}

	// Init Root database
	var err error
	log.Info("Initializing database...")
	if db.Root, err = db.Initiate(service.RootDatabaseInitializer); err != nil {
		log.Error(err)
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

	// initialize statistics repository
	log.Info("Initializing statistics repository...")
	if err := statistics.InitializeStatisticRepos(); err != nil {
		log.Error(err)
		os.Exit(-6)
	}

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
