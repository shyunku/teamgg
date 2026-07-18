package main

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	log "github.com/shyunku-libraries/go-logger"
	"math/rand"
	"os"
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
	if err := godotenv.Load(); err != nil {
		log.Error(err)
		os.Exit(-1)
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
	if core.DebugMode {
		log.Debug("Running in debug mode...")
	} else {
		log.Info("Running in production mode...")
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
	statistics.InitializeStatisticRepos()

	// start statistics repository loop
	log.Info("Starting statistics repository loops...")
	//	go statistics.ChampionDetailStatisticsRepo.Loop()
	//	go statistics.TierStatisticsRepo.Loop()
	//	go statistics.MasteryStatisticsRepo.Loop()

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
