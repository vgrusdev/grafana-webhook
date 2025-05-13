package main

import (
	"os"
	"os/signal"
	"strconv"
	"log/slog"
	"context"
	"time"
)

func main() {
	a := App{}

	
	var botToken string
	var chatID   int64
	//var err		 error

	botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if len(botToken) == 0 {
		slog.Error("TELEGRAM_BOT_TOKEN env is not set.")
		return
	}
	chatID_s := os.Getenv("TELEGRAM_CHAT_ID")
	if len(chatID_s) == 0 {
		slog.Warn("TELEGRAM_CHAT_ID env is not set. Use ChatID Label in Grafana Alerts.")
		chatID = -1
	} else {
		chatID, err := strconv.ParseInt(chatID_s, 10, 64)
		if err != nil {
			slog.Error("TELEGRAM_CHAT_ID env is not integer. Use -1 if you wand to use ChatID Label in Grafana Alerts.")
			return
		}
	}


	ctxBot, cancelBot := context.WithCancel(context.Background())
    defer cancelBot()

	err := a.Initialize(ctxBot, botToken, chatID, "4000")
	if err != nil {
		slog.Error("Init", "err", err)
		os.Exit(1)
	}

	go a.Run()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctxSrv, cancelSrv := context.WithTimeout(context.Background(), 8 * time.Second)
	defer cancelSrv()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	a.Shutdown(ctxSrv)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.

	os.Exit(0)

}
