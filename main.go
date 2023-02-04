package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/slack-go/slack"
)

func getChannels(client *slack.Client) []slack.Channel {
	channels, _, err := client.GetConversationsForUser(&slack.GetConversationsForUserParameters{})
	if err != nil {
		fmt.Println(err)
	}
	return channels
}

func postStartMessage(client *slack.Client) string {
	_, ts, err := client.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText("タスク実行を開始します", true))
	if err != nil {
		fmt.Println(err)
	}
	return ts
}

func postEndMessage(client *slack.Client, start time.Time, ts string, count int) {
	duration := time.Now().Sub(start)
	avg := count / int(duration)
	message := "タスク実行を終了します\n" + duration.String() + "\n" + "count:" + strconv.FormatInt(int64(count), 10) + "\n" + "avg:" + strconv.FormatInt(int64(avg), 10)
	_, _, err := client.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, true), slack.MsgOptionTS(ts), slack.MsgOptionBroadcast())
	if err != nil {
		fmt.Println(err)
	}
}

func deleteMessages(client *slack.Client, channels []slack.Channel, now time.Time, daysStr string) int {
	count := 0
	days, _ := strconv.Atoi(daysStr)
	for i := range channels {
		id := channels[i].ID
		latest := strconv.FormatInt(now.AddDate(0, 0, -days).Unix(), 10)
		params := slack.GetConversationHistoryParameters{ChannelID: id, Limit: 1000, Latest: latest}
		res, _ := client.GetConversationHistory(&params)
		for j := range res.Messages {
			count++
			ts := res.Messages[j].Msg.Timestamp
			_, _, err := client.DeleteMessage(id, ts)
			if err != nil {
				fmt.Println(id + ":" + ts + ":" + err.Error())
				if err.Error() != "message_not_found" {
					fmt.Println(err)
					time.Sleep(time.Second * 1)
					recover()
				} else {
					fmt.Println(err)
				}
			}
		}
	}
	return count
}

func main() {
	botClient := slack.New(os.Getenv("SLACK_OAUTH_BOT_TOKEN"))
	userClient := slack.New(os.Getenv("SLACK_OAUTH_USER_TOKEN"))
	start := time.Now()
	ts := postStartMessage(botClient)
	channels := getChannels(userClient)
	count := deleteMessages(userClient, channels, start, os.Args[1])
	postEndMessage(botClient, start, ts, count)
}
