package service

import (
	log "github.com/shyunku-libraries/go-logger"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Finalizer func() error

func PrepareFinalize(cancel func(), wg *sync.WaitGroup, finalizers []Finalizer) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGTERM:
			log.Info("SIGTERM signal received, shutting down.")
			finalize(cancel, wg, finalizers)
			return
		case syscall.SIGINT:
			log.Info("SIGINT signal received, shutting down.")
			finalize(cancel, wg, finalizers)
		default:
			log.Warn("Received unexpected signal:", sig)
		}
	}
}

// finalize process for graceful shutdown
func finalize(cancel func(), wg *sync.WaitGroup, finalizers []Finalizer) {
	log.Info("finalizing application for graceful shutdown...")
	cancel()

	// wait for all goroutines to finish
	log.Info("waiting for all goroutines to finish...")
	wg.Wait()

	// finalize (resource release, etc.)
	for _, f := range finalizers {
		if err := f(); err != nil {
			log.Error("error during finalization:", err)
		}
	}
	log.Info("application shutdown complete.")
	// shutdown
	os.Exit(0)
}
