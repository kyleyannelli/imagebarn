package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	"kmfg.dev/imagebarn/v1/web"
)

const PORT = 30109

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(fmt.Errorf("Unable to start ImageBarn: %v", err))
	}
	setupLogs()

	// the passing of the channel and wait group down so far feels a little messy
	signalChain := make(chan os.Signal, 1)
	signal.Notify(signalChain, syscall.SIGINT, syscall.SIGTERM)
	stopChan := make(chan struct{})
	wg := sync.WaitGroup{}

	go web.StartServer(PORT, stopChan, &wg)

	sig := <-signalChain
	slog.Info(fmt.Sprintf("Received signal: %s, initiating shutdown...", sig))
	close(stopChan)

	wg.Wait()
	slog.Info("All routines have safely stopped... Exiting!")
}

func setupLogs() *slog.Logger {
	w := os.Stdout

	logger := slog.New(
		tint.NewHandler(w, &tint.Options{
			AddSource:  true,
			TimeFormat: time.ANSIC,
			Level:      slog.LevelDebug,
		}),
	)

	slog.SetDefault(logger)

	return logger
}
