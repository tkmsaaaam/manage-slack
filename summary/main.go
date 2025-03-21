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

	channelById := map[string]slack.Channel{}
	for _, channel := range conversations {
		channelById[channel.ID] = channel
	}

	countBySiteByChannel, countByHost, countBychannel := c.makeResult(conversations)

	message := c.createMessage(countBySiteByChannel, countBychannel, channelById)
	botClient := slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	_, _, err := botClient.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, false))
	if err != nil {
		log.Println("can not post:", err)
	}
	sendMetrics(countByHost, countBychannel, channelById)
}

func (c *config) getConversationsForUser() []slack.Channel {
	conversations, _, err := c.userClient.GetConversationsForUser(&slack.GetConversationsForUserParameters{})
	if err != nil {
		log.Println("can not get channels:", err)
	}
	return conversations
}

func (c *config) makeResult(conversations []slack.Channel) (map[string]map[string]int, map[string]int, map[string]int) {
	latest := strconv.FormatInt(time.Date(c.now.Year(), c.now.Month(), c.now.Day(), 0, 0, 0, 0, c.now.Location()).Unix(), 10)
	oldest := strconv.FormatInt(time.Date(c.yesterDay.Year(), c.yesterDay.Month(), c.yesterDay.Day(), 0, 0, 0, 0, c.yesterDay.Location()).Unix(), 10)

	countBySiteByChannel := map[string]map[string]int{}
	countByHost := map[string]int{}
	countByChannel := map[string]int{}

	for _, conversation := range conversations {
		params := slack.GetConversationHistoryParameters{ChannelID: conversation.ID, Limit: 1000, Latest: latest, Oldest: oldest}
		conversationHistory, err := c.userClient.GetConversationHistory(&params)
		if err != nil {
			log.Println("can not get history channelID:", conversation.ID, err)
			continue
		}

		i := len(conversationHistory.Messages)
		countByUser := map[string]int{}
		for _, message := range conversationHistory.Messages {
			i += message.ReplyCount

			if strings.HasPrefix(message.Msg.Text, "<http") {
				url, err := url.Parse(strings.Split(message.Msg.Text[1:], "|")[0])
				if err == nil && url.Host != "" {
					countByHost[url.Host] += 1
				}
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
			countByUser[userName] += 1
		}

		countByChannel[conversation.ID] = i
		countBySiteByChannel[conversation.ID] = countByUser
	}
	return countBySiteByChannel, countByHost, countByChannel
}

func (c *config) createMessage(countBySiteByChannel map[string]map[string]int, countByChannel map[string]int, channelById map[string]slack.Channel) string {
	count := 0
	for _, v := range countByChannel {
		count += v
	}
	var message = c.yesterDay.Format("2006-01-02") + "\n" + c.yesterDay.Format("Monday") + "\n" + strconv.FormatInt(int64(count), 10) + "\n"
	for _, channel := range channelById {
		mapBySite, ok := countBySiteByChannel[channel.ID]
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

func sendMetrics(countByHost, countByChannel map[string]int, channelById map[string]slack.Channel) {
	url := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
	if url == "" {
		return
	}
	for k, v := range countByHost {
		send(url, k, "host", v)
	}
	for _, v := range channelById {
		i := 0
		if v, ok := countByChannel[v.ID]; ok {
			i = v
		}
		send(url, v.Name, "channel", i)
	}
}

func send(url, k, grouping string, v int) {
	n := strings.ReplaceAll(strings.ReplaceAll(k, ".", "_"), "-", "_")
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "slack",
		Name:        n,
		Help:        k + " messages count by " + grouping,
		ConstLabels: prometheus.Labels{"pusher": "slack-daily", "grouping": grouping},
	})
	counter.Add(float64(v))
	if err := push.New(url, n).Collector(counter).Push(); err != nil {
		log.Println("can not push", err)
	}
}
