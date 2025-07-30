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
	var err		 error
	var tMethod	string

	botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if len(botToken) == 0 {
		slog.Error("TELEGRAM_BOT_TOKEN env is not set.")
		return
	}
	chatID_s := os.Getenv("TELEGRAM_CHAT_ID")
	if len(chatID_s) == 0 {
		slog.Warn("TELEGRAM_CHAT_ID env is not set. Use \"chatID\" Label in Grafana Alerts to assign Telegram bot chatID.")
		chatID = -1
	} else {
		chatID, err = strconv.ParseInt(chatID_s, 10, 64)
		if err != nil {
			slog.Error("TELEGRAM_CHAT_ID env is not integer. Use -1 if you want to use \"chatID\" Label in Grafana Alerts.")
			return
		}
	}
	tMethod = os.Getenv("TELEGRAM_METHOD")
	if (len(tMethod) == 0) || ((tMethod != "DIRECT") && (tMethod != "ATCLIENT")) {
		slog.Warn("TELEGRAM_METHOD env is not set or incorrect. Options are: \"DIRECT\"(d) or \"ATCLIENT\". It can be owerwritten by \"tMethod\" Label in Grafana Alert.")
		tMethod = "DIRECT"
	} else {
		slog.Info("tMethod is set.", "TELEGRAM_METHOD", tMethod)
	}

	whPort := os.Getenv("WEBHOOK_PORT")
	if len(whPort) == 0 {
		slog.Warn("WEBHOOK_PORT env is not set. Use default port 4000")
		whPort = "4000"
	}

	var myMinio *myMinio_t

	myMinio = &myMinio_t {
		host:	os.Getenv("MINIO_HOST"),
		port:	os.Getenv("MINIO_PORT"),
		key:	os.Getenv("MINIO_KEY"),
		secret:	os.Getenv("MINIO_SECRET"),
	}

	// bot context with cancel func
	ctxBot, cancelBot := context.WithCancel(context.Background())
    defer cancelBot()

	err = a.Initialize(ctxBot, botToken, chatID, whPort, myMinio, tMethod)
	if err != nil {
		slog.Error("Init", "err", err)
		cancelBot()
		os.Exit(1)
	}

	chSrv := make(chan string)
	// run srv.ListenAndServe()
	go a.Run(chSrv)

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	select {	// which channel will be unblocked first ?
	case <-c:	// os.Interrupt

		// Create a deadline to wait for.
		ctxSrv, cancelSrv := context.WithTimeout(context.Background(), 8 * time.Second)
		defer cancelSrv()

		// Doesn't block if no connections, but will otherwise wait
		// until the timeout deadline.
		a.Shutdown(ctxSrv)
		// Optionally, you could run srv.Shutdown in a goroutine and block on
		// <-ctx.Done() if your application should wait for other services
		// to finalize based on context cancellation.

		// wait for srv.shutdown results
		s, ok := <-chSrv
		if ok == true {
			slog.Info(s)
		}

	case s := <-chSrv:	// srv.ListenAndServe ended itself, probably due to error.
		slog.Error(s)
	}

}