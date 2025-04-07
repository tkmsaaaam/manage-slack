package main

import (
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

func main() {
	userClient := &SlackClient{slack.New(os.Getenv("SLACK_USER_TOKEN"))}

	targetUrl := os.Getenv("TARGET_URL")
	if targetUrl == "" {
		log.Println("TARGET_URL is not set")
		return
	}
	e := strings.Split(targetUrl, "/")
	if len(e) < 5 {
		log.Println("TARGET_URL is not valid")
		return
	}

	channelID := e[4]
	t := strings.ReplaceAll(e[5], "p", "")
	timestamp := t[:10]+"."+t[10:]

	messages, _, _, err := userClient.GetConversationReplies(&slack.GetConversationRepliesParameters{ChannelID: channelID, Timestamp: timestamp})
	if err != nil {
		log.Println("Can not get messages:", err)
		return
	}
	if len(messages) == 0 {
		log.Println("No replies found")
		return
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

		if t.After(time.Now().AddDate(0, 0, -1)) {
			log.Println(targetUrl)
			break
		}
	}
}
