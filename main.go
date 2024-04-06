package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/slack-go/slack"
)

type SlackClient struct {
	*slack.Client
}

func (client SlackClient) getChannels() []slack.Channel {
	channels, _, err := client.GetConversationsForUser(&slack.GetConversationsForUserParameters{})
	if err != nil {
		log.Printf("Can not get channels: %v\n", err)
	}
	return channels
}

func (client SlackClient) postStartMessage() string {
	_, ts, err := client.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText("タスク実行を開始します", true))
	if err != nil {
		log.Printf("Can not post start message: %v\n", err)
	}
	return ts
}

func (client SlackClient) postEndMessage(start time.Time, ts string, messageCount int, fileCount int) {
	duration := time.Since(start)
	avg := float64(messageCount) / duration.Seconds()
	message := "タスク実行を終了します\n" + duration.String() + "\n" + "message count: " + strconv.FormatInt(int64(messageCount), 10) + "\n" + "avg: " + strconv.FormatFloat(avg, 'f', -1, 64) + "/s" + "\n" + "file count: " + strconv.FormatInt(int64(fileCount), 10)
	_, _, err := client.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, true), slack.MsgOptionTS(ts), slack.MsgOptionBroadcast())
	if err != nil {
		log.Printf("End message can not post: %v\n", err)
	}
}

func (client SlackClient) deleteMessage(id string, ts string) {
	_, _, err := client.DeleteMessage(id, ts)
	if err != nil {
		log.Printf("Can not delete message: %s : %s : %v\n", id, ts, err)
		if err.Error() != "message_not_found" {
			recover()
		}
	}
}

func makeDays(daysStr string) int {
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		log.Printf("env DAYS is invalid: %v\n", err)
		const DAFAULT_DAYS = 3
		return DAFAULT_DAYS
	}
	return days
}

func (client SlackClient) loopInAllChannels(channels []slack.Channel, now time.Time, days int) int {
	count := 0
	for _, channel := range channels {
		id := channel.ID
		latest := strconv.FormatInt(now.AddDate(0, 0, -days).Unix(), 10)
		params := slack.GetConversationHistoryParameters{ChannelID: id, Limit: 1000, Latest: latest}
		res, err := client.GetConversationHistory(&params)
		if err != nil {
			log.Printf("Can not get history: %v\n", err)
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
					log.Printf("Can not get replies: %v\n", err)
				}
				for _, reply := range replies {
					count++
					client.deleteMessage(id, reply.Msg.Timestamp)
				}
			}
			client.deleteMessage(id, message.Msg.Timestamp)
		}
	}
	return count
}

func (client SlackClient) deleteFiles(now time.Time, days int) int {
	latest := now.AddDate(0, 0, -days).Unix()
	params := slack.GetFilesParameters{TimestampTo: slack.JSONTime(latest)}
	res, _, err := client.GetFiles(params)
	count := 0
	if err != nil {
		log.Printf("Can not get file: %v\n", err)
		return count
	}
	for _, file := range res {
		id := file.ID
		err := client.DeleteFile(id)
		if err != nil {
			log.Printf("Can not delete file: %v\n", err)
			continue
		}
		count++
	}
	return count
}

func main() {
	botClient := slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	userClient := slack.New(os.Getenv("SLACK_USER_TOKEN"))
	start := time.Now()
	ts := SlackClient{botClient}.postStartMessage()
	channels := SlackClient{userClient}.getChannels()
	daysStr := os.Getenv("DAYS")
	days := makeDays(daysStr)
	messageCount := SlackClient{userClient}.loopInAllChannels(channels, start, days)
	fileCount := SlackClient{botClient}.deleteFiles(start, days)
	SlackClient{botClient}.postEndMessage(start, ts, messageCount, fileCount)
}
