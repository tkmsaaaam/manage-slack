package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/slack-go/slack"
)

type SlackClient struct {
	*slack.Client
}

func (client *SlackClient) getChannels() ([]slack.Channel, error) {
	channels, _, err := client.GetConversationsForUser(&slack.GetConversationsForUserParameters{})
	if err != nil {
		return nil, fmt.Errorf("can not get channels: %w", err)
	}
	return channels, nil
}

func (client *SlackClient) postStartMessage() string {
	_, ts, err := client.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText("タスク実行を開始します", true))
	if err != nil {
		log.Println("Can not post start message:", err)
	}
	return ts
}

func (client *SlackClient) postEndMessage(duration time.Duration, ts string, messageCount, fileCount int) {
	avg := float64(messageCount) / duration.Seconds()
	message := "タスク実行を終了します\n" + duration.String() + "\n" + "message count: " + strconv.FormatInt(int64(messageCount), 10) + "\n" + "avg: " + strconv.FormatFloat(avg, 'f', -1, 64) + "/s" + "\n" + "file count: " + strconv.FormatInt(int64(fileCount), 10)
	_, _, err := client.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, true), slack.MsgOptionTS(ts), slack.MsgOptionBroadcast())
	if err != nil {
		log.Println("End message can not post:", err)
	}
}

func (client *SlackClient) deleteMessage(id, ts string) {
	_, _, err := client.DeleteMessage(id, ts)
	if err != nil {
		log.Println("Can not delete message:", id, ":", ts, ":", err)
		if err.Error() != "message_not_found" {
			recover()
		}
	}
}

func makeDays(daysStr string) int {
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		log.Println("env DAYS is invalid:", err)
		const DAFAULT_DAYS = 3
		return DAFAULT_DAYS
	}
	return days
}

func (client *SlackClient) loopInAllChannels(channels []slack.Channel, now time.Time, days int) int {
	count := 0
	for _, channel := range channels {
		id := channel.ID
		latest := strconv.FormatInt(now.AddDate(0, 0, -days).Unix(), 10)
		params := slack.GetConversationHistoryParameters{ChannelID: id, Limit: 1000, Latest: latest}
		res, err := client.GetConversationHistory(&params)
		if err != nil {
			log.Println("Can not get history:", err)
			continue
		}
		for _, message := range res.Messages {
			if len(message.Reactions) > 0 {
				continue
			}
			count++
			if message.ReplyCount != 0 {
				repliesParams := slack.GetConversationRepliesParameters{ChannelID: id, Timestamp: message.Msg.Timestamp}
				replies, _, _, err := client.GetConversationReplies(&repliesParams)
				if err != nil {
					log.Println("Can not get replies:", err)
				} else {
					for _, reply := range replies {
						count++
						client.deleteMessage(id, reply.Msg.Timestamp)
					}
				}
			}
			client.deleteMessage(id, message.Msg.Timestamp)
		}
	}
	return count
}

func (client *SlackClient) deleteFiles(now time.Time, days int) int {
	latest := now.AddDate(0, 0, -days).Unix()
	params := slack.GetFilesParameters{TimestampTo: slack.JSONTime(latest)}
	res, _, err := client.GetFiles(params)
	count := 0
	if err != nil {
		log.Println("Can not get file:", err)
		return count
	}
	for _, file := range res {
		id := file.ID
		err := client.DeleteFile(id)
		if err != nil {
			log.Println("Can not delete file:", err)
			continue
		}
		count++
	}
	return count
}

func sendCounter(url, k string, v int) {
	n := strings.ReplaceAll(strings.ReplaceAll(k, ".", "_"), "-", "_")
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "slack",
		Name:        n,
		ConstLabels: prometheus.Labels{"pusher": "slack-remover"},
	})
	counter.Add(float64(v))
	if err := push.New(url, n).Collector(counter).Push(); err != nil {
		log.Println("can not push", k, err)
	}
}

func main() {
	botClient := &SlackClient{slack.New(os.Getenv("SLACK_BOT_TOKEN"))}
	userClient := &SlackClient{slack.New(os.Getenv("SLACK_USER_TOKEN"))}
	start := time.Now()
	ts := botClient.postStartMessage()
	channels, err := userClient.getChannels()
	if err != nil {
		log.Println("Can not get channels", err)
		return
	}
	daysStr := os.Getenv("DAYS")
	days := makeDays(daysStr)
	messageCount := userClient.loopInAllChannels(channels, start, days)
	fileCount := botClient.deleteFiles(start, days)
	duration := time.Since(start)
	botClient.postEndMessage(duration, ts, messageCount, fileCount)
	url := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
	if url == "" {
		return
	}
	sendCounter(url, "deleted_messages", messageCount)
	sendCounter(url, "deleted_files", fileCount)
	requestDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "remover_duration_seconds",
		Namespace:   "slack",
		ConstLabels: prometheus.Labels{"pusher": "slack-remover"},
	})
	requestDuration.Observe(duration.Seconds())
	if err := push.New(url, "remover_duration_seconds").Collector(requestDuration).Push(); err != nil {
		log.Println("can not push remover_duration_seconds", err)
	}
}
