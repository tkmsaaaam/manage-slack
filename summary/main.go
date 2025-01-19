package main

import (
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/slack-go/slack"
)

type config struct {
	userClient *slack.Client
	now        time.Time
	yesterDay  time.Time
}

func main() {
	userClient := slack.New(os.Getenv("SLACK_USER_TOKEN"))

	now := time.Now()
	yesterDay := now.AddDate(0, 0, -1)

	c := &config{userClient: userClient, now: now, yesterDay: yesterDay}
	conversations := c.getConversationsForUser()

	channelMap := makeChannelMap(conversations)

	mapBySiteByChannel, mapByHost, mapByChannel := c.createChannels(conversations)

	message := c.createMessage(mapBySiteByChannel, mapByChannel, channelMap)
	botClient := slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	_, _, err := botClient.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, false))
	if err != nil {
		log.Printf("can not post: %v\n", err)
	}
	sendMetrics(mapByHost, mapByChannel, channelMap)
}

func (c *config) getConversationsForUser() []slack.Channel {
	conversations, _, err := c.userClient.GetConversationsForUser(&slack.GetConversationsForUserParameters{})
	if err != nil {
		log.Printf("can not get channels: %v\n", err)
	}
	return conversations
}

func (c *config) createChannels(conversations []slack.Channel) (map[string]map[string]int, map[string]int, map[string]int) {
	latest := strconv.FormatInt(time.Date(c.now.Year(), c.now.Month(), c.now.Day(), 0, 0, 0, 0, c.now.Location()).Unix(), 10)
	oldest := strconv.FormatInt(time.Date(c.yesterDay.Year(), c.yesterDay.Month(), c.yesterDay.Day(), 0, 0, 0, 0, c.yesterDay.Location()).Unix(), 10)

	mapBySiteByChannel := map[string]map[string]int{}
	mapByHost := map[string]int{}
	mapByChannel := map[string]int{}

	for _, conversation := range conversations {
		params := slack.GetConversationHistoryParameters{ChannelID: conversation.ID, Limit: 1000, Latest: latest, Oldest: oldest}
		conversationHistory, err := c.userClient.GetConversationHistory(&params)
		if err != nil {
			log.Printf("can not get history channelID: %s, %v\n", conversation.ID, err)
			continue
		}

		i := len(conversationHistory.Messages)
		mapByUserName := map[string]int{}
		for _, message := range conversationHistory.Messages {
			i += message.ReplyCount

			if strings.HasPrefix(message.Msg.Text, "<http") {
				url, error := url.Parse(strings.Split(message.Msg.Text[1:], "|")[0])
				if error != nil {
					continue
				}
				mapByHost[url.Host] += 1
			}

			userName := ""
			if message.Msg.Username != "" {
				userName = message.Msg.Username
			} else if message.BotProfile != nil && message.BotProfile.Name != "" {
				userName = message.BotProfile.Name
			}
			if userName == "" {
				continue
			}
			mapByUserName[userName] += 1
		}

		mapByChannel[conversation.ID] = i
		mapBySiteByChannel[conversation.ID] = mapByUserName
	}
	return mapBySiteByChannel, mapByHost, mapByChannel
}

func makeChannelMap(channels []slack.Channel) map[string]slack.Channel {
	channelMap := map[string]slack.Channel{}
	for _, channel := range channels {
		channelMap[channel.ID] = channel
	}
	return channelMap
}

func (c *config) createMessage(mapBySiteByChannel map[string]map[string]int, mapByChannel map[string]int, channelMap map[string]slack.Channel) string {
	count := 0
	for _, v := range mapByChannel {
		count += v
	}
	var message = c.yesterDay.Format("2006-01-02") + "\n" + c.yesterDay.Format("Monday") + "\n" + strconv.FormatInt(int64(count), 10) + "\n"
	for _, channel := range channelMap {
		mapBySite, ok := mapBySiteByChannel[channel.ID]
		if !ok {
			continue
		}
		if len(mapBySite) == 0 {
			continue
		}
		message += "\n<#" + channel.ID + ">\n"
		for k, v := range mapBySite {
			message += k + " : " + strconv.FormatInt(int64(v), 10) + "\n"
		}
	}
	return message
}

func sendMetrics(mapByHost map[string]int, mapByChannel map[string]int, channelMap map[string]slack.Channel) {
	url := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
	if url == "" {
		return
	}
	for k, v := range mapByHost {
		n := strings.ReplaceAll(strings.ReplaceAll(k, ".", "_"), "-", "_")
		counter := prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "slack",
			Name:        n,
			Help:        k + " messages count by host",
			ConstLabels: prometheus.Labels{"pusher": "slack-daily", "grouping": "host"},
		})
		counter.Add(float64(v))
		if err := push.New(url, n).Collector(counter).Push(); err != nil {
			log.Println("can not push", err)
		}
	}
	for k, v := range mapByChannel {
		name := k
		name = channelMap[k].Name
		n := strings.ReplaceAll(name, "-", "_")
		counter := prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "slack",
			Name:        n,
			Help:        k + " messages count by channel",
			ConstLabels: prometheus.Labels{"pusher": "slack-daily", "grouping": "channel"},
		})
		counter.Add(float64(v))
		if err := push.New(url, n).Collector(counter).Push(); err != nil {
			log.Println("can not push", err)
		}
	}
}
