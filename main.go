package main

import (
	"os"
	"os/signal"
	"strconv"
	"log/slog"
	"context"
	"time"
	"strings"
)

func main() {

	a := App{}

	
	var botToken string
	var chatID   int64
	var err		 error

	// Bot Token. Required parameter. Allowed options are:
	// "ATCLIENT" - in the case external java scipt with embedded bot parameters is used to send Telegram messages,
	//				do not forget to setup ATCLIENT_xxx env
	// "bot_token" - string for Direct to Telegram sending.
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

	whPort := os.Getenv("WEBHOOK_PORT")
	if len(whPort) == 0 {
		slog.Warn("WEBHOOK_PORT env is not set. Use default port 4000")
		whPort = "4000"
	}

	var myMinio *myMinio_t

	// Grafana needs to have options to save rendered images in teh S3 (MIMIO) storage.
	// Setup MINIO server parameters to have images processed.
	myMinio = &myMinio_t {
		host:	os.Getenv("MINIO_HOST"),
		port:	os.Getenv("MINIO_PORT"),
		key:	os.Getenv("MINIO_KEY"),
		secret:	os.Getenv("MINIO_SECRET"),
	}

	// Initialise atClient

	var atClient *atClient_t

	if botToken == "ATCLIENT" {
		atClient = &atClient_t {
			javaPath: "java",
			//javaParam: []string{},
			//jarPath: "atclient.jar",
			//botServer: "botserver",
			//port: "8888",
			timeout: 1*time.Second,
		}
		
		env := os.Getenv("ATCLIENT_JAVAPATH")
		if len(env) > 0 {
			atClient.javaPath = env
		}

		javaArgs 	:= []string{}

		// atClient default values
		javaParam 	:= []string{"-Xmx2048m", "-Dfile.encoding=UTF-8"}
		jarPath 	:= "atclient.jar"
		className 	:= ""			// Depriciated parameter of the function, restore it if needed in the future.
		botServer 	:= "botserver"
		port 		:= "8888"

		//-------
		env = os.Getenv("ATCLIENT_PARAM")
		if len(env) > 0 {
			javaParam = strings.FieldsFunc(env, func(c rune) bool {
														return c == ' ' || c == ',' || c == ';'
														})
			javaArgs = append(javaArgs, javaParam...)
		}

		//--------
		env = os.Getenv("ATCLIENT_JARPATH")
		if len(env) > 0 {
			jarPath = env
		}
		if className == "" {
			javaArgs = append(javaArgs, "-jar", jarPath)
		} else {
			javaArgs = append(javaArgs, "-cp", jarPath, className)
		}

		//--------
		env = os.Getenv("ATCLIENT_BOTSERVER")
		if len(env) > 0 {
			botServer = env
		}
		env = os.Getenv("ATCLIENT_PORT")
		if len(env) > 0 {
			port = env
		}
		atClient.javaParam = append(javaArgs, "\"" + botServer + "\"", "\"" + port + "\"")

		env = os.Getenv("ATCLIENT_TIMEOUT")
		if len(env) > 0 {
			t, err := time.ParseDuration(env)
			if err != nil {
				slog.Warn("ATCLIENT_TIMEOUT", "time.Duration format error", err)
			} else {
				atClient.timeout = t
			}
		}
	} else {
		atClient = nil
	}


	// bot context with cancel func
	ctxBot, cancelBot := context.WithCancel(context.Background())
    defer cancelBot()

	err = a.Initialize(ctxBot, botToken, chatID, whPort, myMinio, atClient)
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