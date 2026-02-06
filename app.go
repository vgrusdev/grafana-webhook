package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	//"bytes"
	"fmt"
	"os"

	//"io"

	"github.com/gorilla/mux"

	"context"
	"strconv"
	"strings"

	//"errors"
	"bytes"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type App struct {
	//router 	*mux.Router
	srv      *http.Server
	ctx      context.Context
	bot      *bot.Bot
	chatID   int64
	myMinio  *myMinio_t
	atClient *atClient_t
}

type myMinio_t struct {
	host   string
	port   string
	key    string
	secret string
}

type Body struct {
	Receiver          string            `json:"receiver,omitempty"`          // 	Name of the contact point.
	Status            string            `json:"status,omitempty"`            // Current status of the alert, firing or resolved.
	OrgId             int64             `json:"orgId,omitempty"`             // ID of the organization related to the payload.
	Alerts            []*AlertBody      `json:"alerts,omitempty"`            //array of alerts	Alerts that are triggering.
	GroupLabels       map[string]string `json:"groupLabels,omitempty"`       //Labels that are used for grouping, map of string keys to string values
	CommonLabels      map[string]string `json:"commonLabels,omitempty"`      //Labels that all alarms have in common, map of string keys to string values
	CommonAnnotations map[string]string `json:"commonAnnotations,omitempty"` //Annotations that all alarms have in common, map of string keys to string values
	ExternalURL       string            `json:"externalURL,omitempty"`       //External URL to the Grafana instance sending this webhook
	Version           string            `json:"version,omitempty"`           // Version of the payload structure.
	//GroupKey	string					`json:"groupKey,omitempty"`			//Key that is used for grouping
	TruncatedAlerts int64  `json:"truncatedAlerts,omitempty"` //number	Number of alerts that were truncated.
	Title           string `json:"title,omitempty"`           //Custom title. Configurable in webhook settings using notification templates.
	State           string `json:"state,omitempty"`           //State of the alert group (either alerting or ok).
	Message         string `json:"message,omitempty"`         //Custom message. Configurable in webhook settings using notification templates.
}

type AlertBody struct {
	Status       string                 `json:"status,omitempty"`       // Current status of the alert, firing or resolved.
	Labels       map[string]string      `json:"labels,omitempty"`       // Labels that are part of this alert, map of string keys to string values.
	Annotations  map[string]interface{} `json:"annotations,omitempty"`  // Annotations that are part of this alert, map of string keys to string values.
	StartsAt     string                 `json:"startsAt,omitempty"`     // Start time of the alert.
	EndsAt       string                 `json:"endsAt,omitempty"`       // End time of the alert, default value when not resolved is 0001-01-01T00:00:00Z.
	Values       map[string]interface{} `json:"values,omitempty"`       // Values that triggered the current status.
	GeneratorURL string                 `json:"generatorURL,omitempty"` // URL of the alert rule in the Grafana UI.
	Fingerprint  string                 `json:"fingerprint,omitempty"`  // The labels fingerprint, alarms with the same labels will have the same fingerprint.
	SilenceURL   string                 `json:"silenceURL,omitempty"`   // URL to silence the alert rule in the Grafana UI.
	DashboardURL string                 `json:"dashboardURL,omitempty"` // A link to the Grafana Dashboard if the alert has a Dashboard UID annotation.
	ImageURL     string                 `json:"imageURL,omitempty"`     // URL of a screenshot of a panel assigned to the rule that created this notification.
}

func (a *App) Initialize(ctx context.Context, botToken string, chatID int64, addr string, myMinio *myMinio_t, atClient *atClient_t) error {

	if botToken == "ATCLIENT" {
		a.bot = nil
	} else {
		tgURL := os.Getenv("TELEGRAM_URL")
		var mbot *bot.Bot
		var err error
		if len(tgURL) > 0 {
			slog.Info("TELEGRAM_URL has been setup from environment TELEGRAM_URL", tgURL)
			opts := []bot.Option{
				bot.WithServerURL(tgURL),
			}
			mbot, err = bot.New(botToken, opts...)
		} else {
			mbot, err = bot.New(botToken)
		}
		if err != nil {
			return err
		}
		a.bot = mbot
	}
	a.chatID = chatID
	a.ctx = ctx

	router := mux.NewRouter()
	router.HandleFunc("/health", a.HealthCheck).Methods("GET")
	router.HandleFunc("/alert", a.Alert).Methods("POST")      // Use per-Alert annotation, labels, images
	router.HandleFunc("/notify", a.Notify).Methods("POST")    // Use Notification Group Message. Only first Immage if there is any.
	router.HandleFunc("/codepage", a.Codepage).Methods("Get") //

	a.srv = &http.Server{
		Handler:      router,
		Addr:         ":" + addr,
		WriteTimeout: 8 * time.Second,
		ReadTimeout:  8 * time.Second,
	}

	a.myMinio = myMinio
	a.atClient = atClient

	return nil
}

func (a *App) Run(c chan string) {
	slog.Info("Running", "port", a.srv.Addr)

	if err := a.srv.ListenAndServe(); err != nil {
		c <- fmt.Sprintf("%s", err)
	} else {
		c <- "OK"
	}
	close(c)
}

func (a *App) Shutdown(ctx context.Context) {
	slog.Info("Srv shutting down..")
	a.srv.Shutdown(ctx)
}

func (a *App) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) Codepage(w http.ResponseWriter, r *http.Request) {

	slog.Info("New Codepage request", "from", r.RemoteAddr, "Length", strconv.FormatInt(r.ContentLength, 10))

	//defer r.Body.Close()

	fmt.Fprintf(w, "Request method: %s\n", r.Method)
	fmt.Fprintf(w, "Request URL: %s\n", r.URL)
	fmt.Fprintf(w, "Request headers: %v\n", r.Header)

	query := r.URL.Query()
	text := query.Get("text")
	fmt.Fprintf(w, "Text: %s\n", text)
	fmt.Fprintf(w, "Text-q: %q\n", text)

	msg := fmt.Sprintf("%s.%s", text, "message-сообщение")
	fmt.Fprintf(w, "Msg: %s\n", msg)
	fmt.Fprintf(w, "Msg-q: %q\n", msg)
	var err error

	if a.bot == nil { // ATCLIENT
		err = a.atClientTelegram(a.chatID, msg, "")
	} else { // DIRECT
		err = a.directTelegram(a.chatID, msg, "")
	}
	if err != nil {
		fmt.Fprintln(w, "Telegram send error")
		slog.Error("Codepage-Webhook, Telegram send error", "err", err)
	} else {
		fmt.Fprintln(w, "Telegram sent success")
		slog.Info("Codepage-Webhook, Telegram sent success")
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) Alert(w http.ResponseWriter, r *http.Request) {
	// Processes json Body of alerts in the loop one-by-one alert.

	//var m map[string]interface{}
	//var m Body

	slog.Info("New Alert request", "from", r.RemoteAddr, "Length", strconv.FormatInt(r.ContentLength, 10))

	m := &Body{} // top-level body of alerts, containing common labels, links, etc

	/*
		err := json.NewDecoder(r.Body).Decode(m)
		if err != nil {
			slog.Error("Alert", "err", err)
			respondWithJSON(w, http.StatusBadRequest, map[string]string{"result": "error", "message": "Invalid JSON Format"})
			return
		}
	*/

	// Read body if present
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	slog.Debug("requesed body:", string(body))
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber() // This keeps numbers as json.Number

	if err := decoder.Decode(m); err != nil {
		slog.Error("Alert", "err", err)
		respondWithJSON(w, http.StatusBadRequest, map[string]string{"result": "error", "message": "Invalid JSON Format"})
		return
	}

	//fmt.Printf("Decoded body debug: m=%+v\n", *m)
	//fmt.Println("")

	slog.Debug("Alert-Webhook", "Top_level_Body_Common_Labels", *m)
	slog.Debug("Alert-Webhook", "Alerts_Count", len(m.Alerts))

	var msg string
	var stars string
	var annotation bool

	//const tLayout = "02.01 15:04:05 MST"
	const tLayout = "02.01 15:04:05"
	const tYear = "2006"
	const stars_O = "**********************"
	const stars_F = "****** FIRING ! ******"
	const stars_R = "****** Resolving *****"
	const stars_M = "****** Message *******"

	tz, err := time.LoadLocation(os.Getenv("TZ"))
	if err != nil { // Always check errors even if they should not happen.
		tz, _ = time.LoadLocation("")
	}

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
		msg = fmt.Sprintf("%s\n", stars)

		alertName := alert.Labels["alertname"]
		ruleName := alert.Labels["rulename"]
		annotation = true
		if alertName == "DatasourceNoData" {
			msg = fmt.Sprintf("%sПропуск данных для правила \"%s\"\n", msg, ruleName)
			annotation = false
		} else if alertName == "DatasourceError" {
			msg = fmt.Sprintf("%sОшибка связи с сервером \"%s\"\n", msg, ruleName)
			annotation = false
		} else {
			msg = fmt.Sprintf("%s%s\n", msg, alertName)
		}

		ts, _ := time.Parse(time.RFC3339, alert.StartsAt)
		msg = fmt.Sprintf("%sStarts: %s\n", msg, ts.In(tz).Format(tLayout))
		te, _ := time.Parse(time.RFC3339, alert.EndsAt)
		if te.Format(tYear) != "0001" {
			duration := te.Sub(ts)
			msg = fmt.Sprintf("%sEnds  : %s\nElapsed: %s\n", msg, te.In(tz).Format(tLayout), duration)
		}

		valuename, exists := alert.Labels["valuename"]
		if !exists {
			valuename = "A"
		}
		value, exists := alert.Values[valuename]
		if exists {
			msg = fmt.Sprintf("%sValue : %8.2f\n", msg, value)
		}
		ann, ok := alert.Annotations["summary"]
		if annotation && ok {
			msg = fmt.Sprintf("%s%s\n%s", msg, stars_M, ann)
		} else {
			msg = fmt.Sprintf("%s%s", msg, stars_O)
		}
		//fmt.Println(msg)
		slog.Info("   +                      ")
		for _, str := range strings.Split(msg, "\n") {
			slog.Info("   |" + str)
		}
		slog.Info("   +                      ")

		var chatID int64
		chatID = -1

		chatID_s, exists := alert.Labels["chatID"]
		if exists {
			chatID, err = strconv.ParseInt(chatID_s, 10, 64)
			if err != nil {
				slog.Error("Alert-Webhook. Grafana \"chatID\" Label is incorrect.", "err=", err)
				chatID = -1
			}
		} else {
			chatID = a.chatID
		}

		if chatID == -1 {
			slog.Warn("Alert-Webhook. Will not send to Telegram due to incorrect ChatID", "ChatID", strconv.FormatInt(chatID, 10))
			//respondWithJSON(w, http.StatusBadRequest, map[string]string{"result": "error", "message": "Incorrect ChatID"+strconv.FormatInt(chatID, 10)})
			//return
		} else {
			slog.Info("Alert-Webhook. Sending to Telegram", "ChatID", strconv.FormatInt(chatID, 10))

			fileName, err := a.getImageFileMinio(alert)
			if err != nil {
				slog.Error("Alert-Webhook", "err", err)
			} else if len(fileName) == 0 {
				slog.Info("Alert-Webhook, getImage: no Image")
			} else {
				defer os.Remove(fileName)
			}

			if a.bot == nil { // ATCLIENT
				err = a.atClientTelegram(chatID, msg, fileName)
			} else { // DIRECT
				err = a.directTelegram(chatID, msg, fileName)
			}
			if err != nil {
				slog.Error("Alert-Webhook, Telegram send error", "err", err)
				ee := fmt.Sprintf("Telegram send error: %s\n", err)
				respondWithJSON(w, http.StatusBadRequest, map[string]string{"result": "error", "message": ee})
				return
			} else {
				slog.Info("Alert-Webhook, Telegram sent success")
			}
		}
	} // for i, alert := range m.Alerts
	respondWithJSON(w, http.StatusCreated, map[string]string{"result": "success"})
}

func (a *App) Notify(w http.ResponseWriter, r *http.Request) {

	slog.Info("New Alert-Notify request", "from", r.RemoteAddr, "Length", strconv.FormatInt(r.ContentLength, 10))

	m := &Body{}

	err := json.NewDecoder(r.Body).Decode(m)
	if err != nil {
		slog.Error("Notify-Webhook", "err", err)
		respondWithJSON(w, http.StatusBadRequest, map[string]string{"result": "error", "message": "Invalid JSON Format"})
		return
	}

	slog.Debug("Notify-Webhook", "Common_Labels", *m)
	slog.Debug("Notify-Webhook", "Alerts_Count", len(m.Alerts))

	var msg string
	var alertWithImage *AlertBody
	var chatID int64

	//var chatID_s string
	//var exists   bool

	chatID = -1
	alertWithImage = nil
	msg = m.Message

	slog.Info("Notify-Webhook. Search Image URL")
	for i, alert := range m.Alerts {
		slog.Info("Notify-Webhook", "Alert_Num", i+1, "json", *alert)

		if len(alert.ImageURL) > 0 { // Image URL exists !
			alertWithImage = alert

			chatID_s, exists := alert.Labels["chatID"]
			if exists {
				chatID, err = strconv.ParseInt(chatID_s, 10, 64)
				if err != nil {
					slog.Error("Notify-Webhook. Grafana ChatID Label is incorrect.", "err=", err)
					chatID = -1
				}
				//} else {
				//	chatID = -1
			}
			break
		}
		if chatID == -1 {
			chatID_s, exists := alert.Labels["chatID"]
			if exists {
				chatID, err = strconv.ParseInt(chatID_s, 10, 64)
				if err != nil {
					slog.Warn("Notify-Webhook. Grafana ChatID Label is incorrect.", "err=", err)
					chatID = -1
				}
			}
		}
	}
	if chatID == -1 {
		chatID = a.chatID
	}

	//fmt.Println(msg)

	slog.Info("   +                      ")
	for _, str := range strings.Split(msg, "\n") {
		slog.Info("   |" + str)
	}
	slog.Info("   +                      ")

	if chatID == -1 {
		slog.Warn("Notify-Webhook. Will not send to Telegram die to incorrect ChatID", "ChatID", strconv.FormatInt(chatID, 10))
		respondWithJSON(w, http.StatusBadRequest, map[string]string{"result": "error", "message": "Incorrect Telegram chatID"})
		return
	}
	slog.Info("Notify-Webhook. Sending to Telegram", "ChatID", strconv.FormatInt(chatID, 10))

	fileName, err := a.getImageFileMinio(alertWithImage)
	if err != nil {
		slog.Error("Notify-Webhook", "err", err)
	} else if len(fileName) == 0 {
		slog.Info("Notify-Webhook, getImage: no Image")
	} else {
		defer os.Remove(fileName)
	}

	if a.bot == nil { // ATCLIENT
		err = a.atClientTelegram(chatID, msg, fileName)
	} else { // DIRECT
		err = a.directTelegram(chatID, msg, fileName)
	}
	if err != nil {
		slog.Error("Notify-Webhook, Telegram send error", "err", err)
		respondWithJSON(w, http.StatusBadRequest, map[string]string{"result": "error", "message": "Telegram send error"})
	} else {
		slog.Info("Notify-Webhook, Telegram sent success")
		respondWithJSON(w, http.StatusCreated, map[string]string{"result": "success"})
	}

}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func (a *App) getImageFileMinio(alert *AlertBody) (string, error) {
	// return string - 	fileName, do not forget to remove it after being used,
	// 					or "", if there is no image in the alert body.
	// error - in case of error downloading image (except no-Image case, when err = nil)

	if alert == nil {
		//slog.Info("getImage: no Image")
		//return nil, fmt.Errorf("getImage, warning: no Image")
		return "", nil
	}
	imageURL := alert.ImageURL
	if len(imageURL) == 0 {
		//slog.Info("getImage: no Image")
		//return nil, fmt.Errorf("getImage, warning: no Image")
		return "", nil
	}
	u, err := url.Parse(imageURL)
	if err != nil {
		return "", fmt.Errorf("getImage: %w", err)
	}

	host := u.Hostname()
	if len(a.myMinio.host) > 0 {
		host = a.myMinio.host
	}
	port := u.Port()
	if len(a.myMinio.port) > 0 {
		port = a.myMinio.port
	}
	if len(port) > 0 {
		host = host + ":" + port
	}

	// Minio client
	mClient, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(a.myMinio.key, a.myMinio.secret, ""),
		Secure: false,
	})
	if err != nil {
		return "", fmt.Errorf("getImage. Minio client create error. Will not send images: %w", err)
	}

	path := strings.TrimPrefix(u.Path, "/")
	bucket, object, found := strings.Cut(path, "/")
	if found == false {
		return "", fmt.Errorf("getImage, no filename %s", path)
	}
	ss := strings.Split(object, "/")
	filePath := "/tmp/" + ss[len(ss)-1]

	// Picture download from Minio
	err = mClient.FGetObject(a.ctx, bucket, object, filePath, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("getImage, image download: %w", err)
	}
	return filePath, nil
}

func (a *App) directTelegram(chatID int64, msg string, fileName string) error {

	var err error
	var fileData []byte

	if len(fileName) == 0 {
		_, err = a.bot.SendMessage(a.ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   msg,
		})
	} else {
		fileData, err = os.ReadFile(fileName)
		if err != nil {
			slog.Error("directTelegram. file read error", "fileName", fileName, "err", err)
			_, err := a.bot.SendMessage(a.ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   msg,
			})
			return err
		}
		_, err = a.bot.SendPhoto(a.ctx, &bot.SendPhotoParams{
			ChatID:  chatID,
			Photo:   &models.InputFileUpload{Filename: fileName, Data: bytes.NewReader(fileData)},
			Caption: msg,
		})
	}
	return err
}
func (a *App) sendImage(alert *AlertBody, msg string) error {

	imageURL := alert.ImageURL
	if len(imageURL) == 0 {
		slog.Info("no Image")
		_, err := a.bot.SendMessage(a.ctx, &bot.SendMessageParams{
			ChatID: a.chatID,
			Text:   msg,
		})
		return err
	}
	u, err := url.Parse(imageURL)
	if err != nil {
		return err
	}
	host := u.Hostname()
	if len(a.myMinio.host) > 0 {
		host = a.myMinio.host
	}
	port := u.Port()
	if len(a.myMinio.port) > 0 {
		port = a.myMinio.port
	}
	if len(port) > 0 {
		host = host + ":" + port
	}

	// Minio client
	mClient, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(a.myMinio.key, a.myMinio.secret, ""),
		Secure: false,
	})
	if err != nil {
		slog.Error("Minio client create error. Will not send images.", "err", err)
		return err
	}

	path := strings.TrimPrefix(u.Path, "/")
	bucket, object, found := strings.Cut(path, "/")
	if found == false {
		slog.Info("no filename", "Path", path)
		return nil
	}
	ss := strings.Split(object, "/")
	filePath := "/tmp/" + ss[len(ss)-1]

	// Picture download from Minio
	err = mClient.FGetObject(a.ctx, bucket, object, filePath, minio.GetObjectOptions{})
	if err != nil {
		return err
	}

	fileData, errReadFile := os.ReadFile(filePath)
	if errReadFile != nil {
		return errReadFile
	}

	params := &bot.SendPhotoParams{
		ChatID:  a.chatID,
		Photo:   &models.InputFileUpload{Filename: filePath, Data: bytes.NewReader(fileData)},
		Caption: msg,
	}

	_, err = a.bot.SendPhoto(a.ctx, params)

	return err

}

//func (a *App) sendAtclient(alert *AlertBody, msg string) (error) {
//
//
//}

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
