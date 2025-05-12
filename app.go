package main

import (
	"encoding/json"
	//"log"
	"log/slog"
	"net/http"
	"time"
	//"bytes"
	"fmt"

	"github.com/gorilla/mux"

	"context"
	"os"
	"os/signal"
	"strconv"

	"github.com/go-telegram/bot"
	//"github.com/go-telegram/bot/models"

)

type App struct {
	router 	*mux.Router
	ctx		context.Context
	bot		*bot.Bot
	chatID  int64
}

type Body struct {
	Receiver	string			`json:"receiver,omitempty"`		// 	Name of the contact point.
	Status		string			`json:"status,omitempty"`		// Current status of the alert, firing or resolved.
	OrgId		int64			`json:"orgId,omitempty"`		// ID of the organization related to the payload.
	Alerts		[]*AlertBody 	`json:"alerts,omitempty"`	//array of alerts	Alerts that are triggering.
	Version		string			`json:"version,omitempty"`		// Version of the payload structure.
	TruncatedAlerts	int64		`json:"truncatedAlerts,omitempty"`	//number	Number of alerts that were truncated.
	State		string			`json:"state,omitempty"`		//State of the alert group (either alerting or ok).
}

type AlertBody struct {
	Status	string				`	json:"status,omitempty"`		// Current status of the alert, firing or resolved.
	Labels	map[string]string	`json:"labels,omitempty"`	// Labels that are part of this alert, map of string keys to string values.
	Annotations	map[string]interface{}	`json:"annotations,omitempty"`	// Annotations that are part of this alert, map of string keys to string values.
	StartsAt	string				`json:"startsAt,omitempty"`		// Start time of the alert.
	EndsAt		string				`json:"endsAt,omitempty"`		// End time of the alert, default value when not resolved is 0001-01-01T00:00:00Z.
	Values	map[string]interface{}	`json:"values,omitempty"`	// Values that triggered the current status.
	SilenceURL	string				`json:"silenceURL,omitempty"`	// URL to silence the alert rule in the Grafana UI.
	DashboardURL	string			`json:"dashboardURL,omitempty"`	// A link to the Grafana Dashboard if the alert has a Dashboard UID annotation.
	ImageURL	string				`json:"imageURL,omitempty"`		// URL of a screenshot of a panel assigned to the rule that created this notification.
}

func (a *App) Initialize(botToken string, chatID int64 ) (context.CancelFunc, error) {
	
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	a.ctx = ctx

	b, err := bot.New(botToken)
    if err != nil {
        slog.Error("Alert", "err", err)
			return nil, err
    }
	a.bot = b
	a.chatID = chatID

	a.router = mux.NewRouter()
	// a.router.HandleFunc("health", HealthCheck).Methods("GET")
	a.router.HandleFunc("/alert", a.Alert).Methods("POST")

	return cancel, nil
}

func (a *App) Run(addr string) {
	slog.Info("Running", "port", addr)
	srv := &http.Server{
		Handler:      a.router,
		Addr:         ":" + addr,
		WriteTimeout: 8 * time.Second,
		ReadTimeout:  8 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("Listen", "err", err)
	}
}

func (a *App) Alert(w http.ResponseWriter, r *http.Request) {
	//var m map[string]interface{}
	var m Body

	err := json.NewDecoder(r.Body).Decode(&m)
	if err != nil {
		slog.Error("Alert", "err", err)
		respondWithJSON(w, http.StatusBadRequest, map[string]string{"result": "error", "message":"Invalid JSON Format"})
		return
	}

	//fmt.Printf("m=%+v\n", m)
	slog.Info("Alert-Webhook", "Common_Labels", m)
	slog.Info("Alert-Webhook", "Alerts_Count", len(m.Alerts))

	//fmt.Printf("Alerts array len = %d\n", len(m.Alerts))

	var msg string
	var stars string
	
	const tLayout  = "02.01 15:04:05"
	const tYear    = "2006"
	const stars_O  = "**********************"
	const stars_F  = "****** FIRING ! ******"
	const stars_R  = "****** Resolving *****"
	const stars_M  = "****** Message *******"

	for i, alert := range m.Alerts {
		slog.Info("Alert-Webhook", "Alert_Num", i+1, "json", *alert)

		//status := alert.Status
		stars = stars_O
		if alert.Status == "firing" {
			//status = "Firing"
			stars = stars_F
		} else if alert.Status == "resolved" {
			//status = "Resolved"
			stars = stars_R
		}
		ts, _ := time.Parse(time.RFC3339, alert.StartsAt)
		msg = fmt.Sprintf("%s\n%s\nStarts: %s\n", stars, alert.Labels["alertname"], ts.Format(tLayout))
		te, _ := time.Parse(time.RFC3339, alert.EndsAt)
		if (te.Format(tYear) != "0001") {
			duration := te.Sub(ts)
			msg = fmt.Sprintf("%sEnds  : %s\nDuration: %s\n", msg, te.Format(tLayout), duration)
		}
		valuename, exists := alert.Labels["valuename"]
		if !exists {
			valuename = "A"
		}
		value, exists := alert.Values[valuename]
		if exists {
			msg = fmt.Sprintf("%sValue :%f\n", msg, value)
		}
		msg = fmt.Sprintf("%s%s\n%s", msg, stars_M, alert.Annotations["summary"])

		fmt.Println(msg)

		var chatID int64

		chatID = -1
		chatID_s, exists := alert.Labels["chatID"]
		if exists {
			chatID, err = strconv.ParseInt(chatID_s, 10, 64)
			if err != nil {
				slog.Error("Grafana ChatID Label is incorrect.", "err=", err)
				chatID = -1
			}
		} else {
			chatID = a.chatID
		}
		
		fmt.Printf("ChatID=%d\n", chatID)

		a.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   msg,
		})
		
		//b.SendMessage(ctx, &bot.SendMessageParams{
		//	ChatID: 313404961,
		//	Text: "Simple Text",
		//})

		
	}
	respondWithJSON(w, http.StatusCreated, map[string]string{"result": "success"})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
/*
json="	map[
			alerts:[
				map[
					annotations:map[
						summary:CPU system is above 15%
					] 
					dashboardURL:http://10.134.16.103:3000/d/rYddErhsR?from=1745744870000&orgId=1&to=1745748500048 
					endsAt:0001-01-01T00:00:00Z 
					fingerprint:2fcd8bb2b3b23a56 
					generatorURL:http://10.134.16.103:3000/alerting/grafana/fek3uz96jcuf4b/view?orgId=1 
					imageURL:http://minio:9000/mybacket/QZuBKH4o25RwnGnkXz9G.png 
					labels:map[
						alertname:Test1-CPU-System 
						grafana_folder:Test
					] 
					panelURL:http://10.134.16.103:3000/d/rYddErhsR?from=1745744870000&orgId=1&to=1745748500048&viewPanel=3 
					silenceURL:http://10.134.16.103:3000/alerting/silence/new?alertmanager=grafana&matcher=__alert_rule_uid__%3Dfek3uz96jcuf4b&orgId=1 
					startsAt:2025-04-27T13:07:50+03:00 
					status:firing 
					valueString:
						[ var='A' 
						labels={} 
						value=18.199999999999363 ], 
						[ var='C' 
						 labels={} 
						 value=1 ] 
					values:map[
						A:18.199999999999363 
						C:1
					]
				]
			] 
			commonAnnotations:map[
				summary:CPU system is above 15%
			] 
			commonLabels:map[
				alertname:Test1-CPU-System 
				grafana_folder:Test
			] 
			externalURL:http://10.134.16.103:3000/ 
			groupKey:{}/{__grafana_autogenerated__=\"true\"}/{__grafana_receiver__=\"webhook\"}:{alertname=\"Test1-CPU-System\", grafana_folder=\"Test\"} 
			groupLabels:map[
				alertname:Test1-CPU-System 
				grafana_folder:Test
			] 
			message:**Firing**\n\n
			Value: A=18.199999999999363, C=1\n
			     Labels:\n
				  - alertname = Test1-CPU-System\n
				  - grafana_folder = Test\nAnnotations:\n
				  - summary = CPU system is above 15%\n
				  Source: http://10.134.16.103:3000/alerting/grafana/fek3uz96jcuf4b/view?orgId=1\n
				  Silence: http://10.134.16.103:3000/alerting/silence/new?alertmanager=grafana&matcher=__alert_rule_uid__%3Dfek3uz96jcuf4b&orgId=1\n
				  Dashboard: http://10.134.16.103:3000/d/rYddErhsR?from=1745744870000&orgId=1&to=1745748500048\n
				  Panel: http://10.134.16.103:3000/d/rYddErhsR?from=1745744870000&orgId=1&to=1745748500048&viewPanel=3\n 
				  orgId:1 
				  receiver:webhook 
				  state:alerting 
				  status:firing 
				  title:[FIRING:1] 
				  Test1-CPU-System 
				  Test  
				  truncatedAlerts:0 version:1
		]"
*/