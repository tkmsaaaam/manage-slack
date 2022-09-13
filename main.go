package main

import (
	"fmt"
	"github.com/slack-go/slack"
	"os"
	"strconv"
	"time"
)

func getChannels(client *slack.Client) []slack.Channel {
	params := slack.GetConversationsParameters{}
	channels, _, _ := client.GetConversations(&params)
	return channels
}

func postStartMessage(client *slack.Client, start time.Time) string {
	message := "タスク実行を開始します\n" + start.String()
	_, ts, err := client.PostMessage(os.Args[3], slack.MsgOptionText(message, true))
	if err != nil {
		fmt.Println(err)
	}
	return ts
}

func postEndMessage(client *slack.Client, start time.Time, ts string) {
	now := time.Now()
	diff := now.Sub(start)
	message := "タスク実行を終了します\n" + now.String() + "\n" + diff.String()
	_, _, err := client.PostMessage(os.Args[3], slack.MsgOptionText(message, true), slack.MsgOptionTS(ts), slack.MsgOptionBroadcast())
	if err != nil {
		fmt.Println(err)
	}
}

func deleteMessages(client *slack.Client, channels []slack.Channel, now time.Time) {
	for i := range channels {
		id := channels[i].ID
		latest := strconv.FormatInt(now.AddDate(0, 0, -3).Unix(), 10)
		params := slack.GetConversationHistoryParameters{ChannelID: id, Limit: 1000, Latest: latest}
		res, _ := client.GetConversationHistory(&params)
		for j := range res.Messages {
			ts := res.Messages[j].Msg.Timestamp
			_, _, err := client.DeleteMessage(id, ts)
			if err != nil {
				if err.Error() != "message_not_found" {
					time.Sleep(time.Second * 1)
					recover()
				} else {
					fmt.Println(err)
				}
			}
		}
	}
}

func main() {
	botClient := slack.New(os.Args[1])
	start := time.Now()
	ts := postStartMessage(botClient, start)
	userClient := slack.New(os.Args[2])
	channels := getChannels(userClient)
	deleteMessages(userClient, channels, start)
	postEndMessage(botClient, start, ts)
}
