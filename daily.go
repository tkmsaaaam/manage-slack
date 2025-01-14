//go:build ignore

package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"
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

type SlackClient struct {
	*slack.Client
}

func main() {
	userClient := slack.New(os.Getenv("SLACK_USER_TOKEN"))
	conversations := SlackClient{userClient}.getConversationsForUser()

	now := time.Now()
	yesterDay := now.AddDate(0, 0, -1)

	channels, count := createChannels(conversations, now, yesterDay, SlackClient{userClient})

	message := createMessage(yesterDay, channels, count)
	botClient := slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	_, _, err := botClient.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, false))
	if err != nil {
		log.Printf("can not post: %v\n", err)
	}
}

func (client SlackClient) getConversationsForUser() []slack.Channel {
	conversations, _, err := client.GetConversationsForUser(&slack.GetConversationsForUserParameters{})
	if err != nil {
		log.Printf("can not get channels: %v\n", err)
	}
	return conversations
}

func createChannels(conversations []slack.Channel, now time.Time, yesterDay time.Time, userClient SlackClient) ([]Channel, int) {
	latest := strconv.FormatInt(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix(), 10)
	oldest := strconv.FormatInt(time.Date(yesterDay.Year(), yesterDay.Month(), yesterDay.Day(), 0, 0, 0, 0, yesterDay.Location()).Unix(), 10)
	var channels []Channel
	var count int

	hostCount := map[string]int{}
	for _, conversation := range conversations {
		params := slack.GetConversationHistoryParameters{ChannelID: conversation.ID, Limit: 1000, Latest: latest, Oldest: oldest}
		conversationHistory, err := userClient.GetConversationHistory(&params)
		if err != nil {
			log.Printf("can not get history channelID: %s, %v\n", conversation.ID, err)
			continue
		}
		channel := Channel{name: conversation.Name, id: conversation.ID}
		for _, message := range conversationHistory.Messages {
			count++
			addUser(&channel, message, conversationHistory.Messages, &hostCount)
		}
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].name < channels[j].name })
	ctx := context.Background()
	exp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatalln(err)
	}
	meterProvider := metric.NewMeterProvider(metric.WithReader(
		metric.NewPeriodicReader(exp, metric.WithInterval(time.Hour*24)),
	))
	defer meterProvider.Shutdown(ctx)
	meter := meterProvider.Meter("github.com/tkmsaaaam/manage-slack")
	for k, v := range hostCount {
		counter, err := meter.Int64Counter(
			k, nil,
		)
		if err != nil {
			log.Println("Can not make conunter", err)
			continue
		}
		counter.Add(ctx, int64(v))
	}
	return channels, count
}

func createMessage(yesterDay time.Time, channels []Channel, count int) string {
	var message = yesterDay.Format("2006-01-02") + "\n" + yesterDay.Format("Monday") + "\n" + strconv.FormatInt(int64(count), 10) + "\n"
	for _, channel := range channels {
		if len(channel.Sites) == 0 {
			continue
		}
		sort.Slice(channel.Sites, func(i, j int) bool { return channel.Sites[i].count > channel.Sites[j].count })
		message += "\n<#" + channel.id + ">\n"
		for _, site := range channel.Sites {
			message += site.name + " : " + strconv.FormatInt(int64(site.count), 10) + "\n"
		}
	}
	return message
}

func addUser(channel *Channel, message slack.Message, threads []slack.Message, hostCount *map[string]int) {
	var name string
	name = setName(name, message)
	increment(hostCount, message)
	if name == "" {
		for _, thread := range threads {
			if thread.ThreadTimestamp == message.ThreadTimestamp {
				name = setName(name, thread)
				increment(hostCount, message)
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

func increment(hostCount *map[string]int, message slack.Message) {
	if !strings.HasPrefix(message.Msg.Text, "<http") {
		return
	}
	url, error := url.Parse(strings.Split(message.Msg.Text[1:], "|")[0])
	if error != nil {
		return
	}
	if _, ok := (*hostCount)[url.Host]; ok {
		(*hostCount)[url.Host]++
	} else {
		(*hostCount)[url.Host] = 1
	}
}
