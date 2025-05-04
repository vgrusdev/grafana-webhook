package main

import (
	"os"
	"strconv"
)
func main() {
	a := App{}

	var botToken string
	var chatID   int64

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if len(botToken) == 0 {
		slog.Error("TELEGRAM_BOT_TOKEN env is not set.")
		return
	}
	chatID_s := os.Getenv("TELEGRAM_CHAT_ID")
	if len(chatID) == 0 {
		slog.Warn("TELEGRAM_CHAT_ID env is not set. Use ChatID Label in Grafana Alerts.")
		chatID = -1
	} else {
		chatID, err := strconv.ParseInt(chatID_s, 10, 64)
		if err != nil {
			slog.Error("TELEGRAM_CHAT_ID env is not integer. Use -1 if you wand to use ChatID Label in Grafana Alerts.")
			return
		}
	}
	
	cancel, err := a.Initialize(botToken, chatID)
	defer cancel()
	if err != nil {
		return
	}

	a.Run("4000")
}
