package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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
	otelExporterEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
	if otelExporterEndpoint == "" {
		// OTEL_EXPORTER_OTLP_METRICS_ENDPOINT is optional, so no need to log
		return
	}
	_, err := url.Parse(otelExporterEndpoint)
	if err != nil {
		log.Println("can not parse otel url:", err)
		return
	}

	ctx := context.Background()

	protocol := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_PROTOCOL")
	if protocol == "" {
		protocol = os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	}

	isHTTP := strings.Contains(protocol, "http")
	if protocol == "" {
		if strings.HasPrefix(otelExporterEndpoint, "http://") || strings.HasPrefix(otelExporterEndpoint, "https://") {
			isHTTP = true
		}
	}

	var exporter sdkmetric.Exporter
	if isHTTP {
		exporter, err = otlpmetrichttp.New(ctx)
	} else {
		exporter, err = otlpmetricgrpc.New(ctx)
	}
	if err != nil {
		log.Println("failed to create otel exporter:", err)
		return
	}

	reader := sdkmetric.NewPeriodicReader(exporter)
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() {
		if err := provider.Shutdown(ctx); err != nil {
			log.Println("error shutting down meter provider:", err)
		}
	}()
	otel.SetMeterProvider(provider)

	meter := provider.Meter("github.com/tkmsaaaam/manage-slack/summary")

	for k, v := range countByHost {
		send(ctx, meter, k, "host", v)
	}
	for _, v := range channelById {
		if vv, ok := countByChannel[v.ID]; ok {
			send(ctx, meter, v.Name, "channel", vv)
		} else {
			send(ctx, meter, v.Name, "channel", 0)
		}
	}
}

func send(ctx context.Context, meter metric.Meter, k, grouping string, v int) {
	n := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(k, ".", "_"), "-", "_"), "www_", "")
	metricName := "slack_" + n
	counter, err := meter.Int64Counter(metricName,
		metric.WithDescription(k+" messages count by "+grouping),
	)
	if err != nil {
		log.Println("failed to create counter:", metricName, err)
		return
	}
	opts := metric.WithAttributes(
		attribute.String("pusher", "slack-daily"),
		attribute.String("grouping", grouping),
	)
	counter.Add(ctx, int64(v), opts)
}
