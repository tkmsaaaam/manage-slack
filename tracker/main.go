package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

type SlackClient struct {
	*slack.Client
}

type Data struct {
	Threads []Thread `json:"threads"`
}

type Thread struct {
	Url       string `json:"url"`
	ChannelId string `json:"channel_id"`
	ThreadTs  string `json:"thread_ts"`
}

func main() {
	userClient := &SlackClient{slack.New(os.Getenv("SLACK_USER_TOKEN"))}
	targetPath := "data.json"
	dirInfo, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println(targetPath, "が存在しません: ", targetPath, err)
			return
		}
		log.Println(targetPath, "の情報取得に失敗しました: ", err)
		return
	}
	if dirInfo.IsDir() {
		log.Println(targetPath, "はファイルではありません")
		return
	}
	file, err := os.Open(targetPath)
	if err != nil {
		log.Println(targetPath, "を開けませんでした: ", err)
		return
	}
	defer file.Close()

	var data Data
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		log.Println("JSONデコードに失敗しました: ", file, err)
		return
	}

	for _, thread := range data.Threads {
		if thread.Url == "" && thread.ChannelId == "" && thread.ThreadTs == "" {
			log.Println("url, channel_id, and thread_ts are all empty")
			continue
		}
		var channelID = thread.ChannelId
		var timestamp = thread.ThreadTs
		var latest = time.Date(0, 0, -1, 0, 0, 0, 0, time.Local)
		if thread.Url != "" {
			e := strings.Split(thread.Url, "/")
			if len(e) < 5 {
				log.Println("url is not valid:", thread.Url)
			} else {
				channelID = e[4]
				t := strings.ReplaceAll(e[5], "p", "")
				timestamp = t[:10] + "." + t[10:]
			}
		}

		if channelID == "" || timestamp == "" {
			log.Println("channel_id and thread_ts are all empty")
			continue
		}

		messages, _, _, err := userClient.GetConversationReplies(&slack.GetConversationRepliesParameters{ChannelID: channelID, Timestamp: timestamp})
		if err != nil {
			log.Println("Can not get messages:", err)
			continue
		}
		if len(messages) == 0 {
			log.Println("No replies found")
			continue
		}
		for _, message := range messages {
			ts := message.Msg.Timestamp
			timestamp, err := strconv.ParseFloat(ts, 64)
			if err != nil {
				log.Println("Can not parse timestamp:", err)
				continue
			}

			sec := int64(timestamp)
			nsec := int64((timestamp - float64(sec)) * 1e9)
			t := time.Unix(sec, nsec)

			if latest.Before(t) {
				latest = t
			}

			if t.After(time.Now().AddDate(0, 0, -1)) {
				log.Println("updated", thread.Url)
				break
			}
		}
		if latest.Before(time.Now().AddDate(0, 0, -2)) {
			log.Println("too old", thread.Url, "latest", latest)
		}
	}
}
