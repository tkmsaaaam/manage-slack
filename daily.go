package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/slack-go/slack"
)

type User struct {
	name  string
	count int
}

type Channel struct {
	name  string
	Users []User
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
		id := conversation.ID
		params := slack.GetConversationHistoryParameters{ChannelID: id, Limit: 1000, Latest: latest, Oldest: oldest}
		conversationHistory, _ := userClient.GetConversationHistory(&params)
		channel := Channel{name: conversation.Name}
		for _, message := range conversationHistory.Messages {
			count++
			addUser(&channel, message)
		}
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].name < channels[j].name })
	var message string
	message += yesterDay.Format("2006-01-02") + "\n"
	message += strconv.FormatInt(int64(count), 10) + "\n"
	for _, channel := range channels {
		if len(channel.Users) == 0 {
			continue
		}
		sort.Slice(channel.Users, func(i, j int) bool { return channel.Users[i].count > channel.Users[j].count })
		message += "\n" + channel.name + "\n"
		for _, user := range channel.Users {
			message += user.name + " : " + strconv.FormatInt(int64(user.count), 10) + "\n"
		}
	}
	botClient.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, true))
}

func addUser(c *Channel, message slack.Message) {
	var name string
	if message.Msg.Username != "" {
		name = message.Msg.Username
	} else if message.BotProfile != nil {
		name = message.BotProfile.Name
	}
	for i, user := range c.Users {
		if user.name == name {
			c.Users[i] = User{name: user.name, count: user.count + 1}
			return
		}
	}
	c.Users = append(c.Users, User{name: name, count: 1})
}
