package core

import (
	log "github.com/shyunku-libraries/go-logger"
	"os"
	"strings"
	core "team.gg-server/util"
)

var (
	AppServerHost string
	AppServerPort = os.Getenv("APP_SERVER_PORT")
	DebugMode     = false
	DebugOnProd   = false
	IsProduction  = false
	UrgentMode    = false

	RsoClientId          = os.Getenv("RSO_CLIENT_ID")
	RsoClientSecret      = os.Getenv("RSO_CLIENT_SECRET")
	RsoClientCallbackUri = os.Getenv("RSO_CLIENT_CALLBACK_URI")
)

func Preload() error {
	log.Debugf("preload started...")

	// load public ip
	ipv4, err := core.GetPublicIp()
	if err != nil {
		return err
	}

	// load debug mode
	DebugMode = environmentBool("DEBUG")

	// Environment and diagnostic concerns are intentionally separate. A
	// production server may temporarily enable DEBUG without exposing
	// development routes or weakening cookie security.
	IsProduction = environmentBool("IS_PROD")

	// load debug on prod
	DebugOnProd = environmentBool("DEBUG_ON_PRODUCTION")
	if DebugMode {
		DebugOnProd = true
	}

	// load urgent mode
	UrgentMode = environmentBool("URGENT")

	AppServerHost = ipv4
	AppServerPort = os.Getenv("APP_SERVER_PORT")

	RsoClientId = os.Getenv("RSO_CLIENT_ID")
	RsoClientSecret = os.Getenv("RSO_CLIENT_SECRET")
	RsoClientCallbackUri = os.Getenv("RSO_CLIENT_CALLBACK_URI")

	log.Debugf("server is active on public ip: %s:%s", AppServerHost, AppServerPort)
	return nil
}

func environmentBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}
