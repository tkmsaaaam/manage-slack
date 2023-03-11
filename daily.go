//go:build ignore

package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/slack-go/slack"
)

type Site struct {
	name  string
	count int
}

type Channel struct {
	name  string
	id    string
	Sites []Site
}

func main() {
	botClient := slack.New(os.Getenv("SLACK_OAUTH_BOT_TOKEN"))
	userClient := slack.New(os.Getenv("SLACK_OAUTH_USER_TOKEN"))
	conversations, _, err := userClient.GetConversationsForUser(&slack.GetConversationsForUserParameters{})
	if err != nil {
		fmt.Println(err)
	}

	now := time.Now()
	yesterDay := now.AddDate(0, 0, -1)
	oldest := strconv.FormatInt(time.Date(yesterDay.Year(), yesterDay.Month(), yesterDay.Day(), 0, 0, 0, 0, yesterDay.Location()).Unix(), 10)
	latest := strconv.FormatInt(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix(), 10)
	var channels []Channel
	var count int
	for _, conversation := range conversations {
		params := slack.GetConversationHistoryParameters{ChannelID: conversation.ID, Limit: 1000, Latest: latest, Oldest: oldest}
		conversationHistory, _ := userClient.GetConversationHistory(&params)
		channel := Channel{name: conversation.Name, id: conversation.ID}
		for _, message := range conversationHistory.Messages {
			count++
			addUser(&channel, message, conversationHistory.Messages)
		}
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].name < channels[j].name })

	var message = yesterDay.Format("2006-01-02") + "\n" + yesterDay.Format("Monday") + "\n" + strconv.FormatInt(int64(count), 10) + "\n"
	for _, channel := range channels {
		if len(channel.Sites) == 0 {
			continue
		}
		sort.Slice(channel.Sites, func(i, j int) bool { return channel.Sites[i].count > channel.Sites[j].count })
		message += "\n<#" + channel.id + ">\n"
		for _, user := range channel.Sites {
			message += user.name + " : " + strconv.FormatInt(int64(user.count), 10) + "\n"
		}
	}
	botClient.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, false))
}

func addUser(channel *Channel, message slack.Message, threads []slack.Message) {
	var name string
	name = setName(name, message)
	if name == "" {
		for _, thread := range threads {
			if thread.ThreadTimestamp == message.ThreadTimestamp {
				name = setName(name, thread)
			}
		}
	}
	for i, user := range channel.Sites {
		if user.name == name {
			channel.Sites[i] = Site{name: user.name, count: user.count + 1}
			return
		}
	}
	channel.Sites = append(channel.Sites, Site{name: name, count: 1})
}

func setName(name string, message slack.Message) string {
	if message.Msg.Username != "" {
		name = message.Msg.Username
	} else if message.BotProfile != nil && message.BotProfile.Name != "" {
		name = message.BotProfile.Name
	}
	return name
}
