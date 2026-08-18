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
	"go.opentelemetry.io/otel/sdk/resource"
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

	countBySiteByChannel, countByHostByChannel, countBychannel := c.makeResult(conversations)

	message := c.createMessage(countBySiteByChannel, countBychannel, channelById)
	botClient := slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	_, _, err := botClient.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, false))
	if err != nil {
		log.Println("can not post:", err)
	}
	sendMetrics(countByHostByChannel, channelById)
}

func (c *config) getConversationsForUser() []slack.Channel {
	conversations, _, err := c.userClient.GetConversationsForUser(&slack.GetConversationsForUserParameters{})
	if err != nil {
		log.Println("can not get channels:", err)
	}
	return conversations
}

func (c *config) makeResult(conversations []slack.Channel) (map[string]map[string]int, map[string]map[string]int, map[string]int) {
	latest := strconv.FormatInt(time.Date(c.now.Year(), c.now.Month(), c.now.Day(), 0, 0, 0, 0, c.now.Location()).Unix(), 10)
	oldest := strconv.FormatInt(time.Date(c.yesterDay.Year(), c.yesterDay.Month(), c.yesterDay.Day(), 0, 0, 0, 0, c.yesterDay.Location()).Unix(), 10)

	countBySiteByChannel := map[string]map[string]int{}
	countByHostByChannel := map[string]map[string]int{}
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
		countByHost := map[string]int{}
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
		countByHostByChannel[conversation.ID] = countByHost
	}
	return countBySiteByChannel, countByHostByChannel, countByChannel
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

func sendMetrics(countByHostByChannel map[string]map[string]int, channelById map[string]slack.Channel) {
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

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "manage-slack/summary"),
		),
	)
	if err != nil {
		log.Println("failed to create resource:", err)
	}

	reader := sdkmetric.NewPeriodicReader(exporter)
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)
	defer func() {
		if err := provider.Shutdown(ctx); err != nil {
			log.Println("error shutting down meter provider:", err)
		}
	}()
	otel.SetMeterProvider(provider)

	meter := provider.Meter("github.com/tkmsaaaam/manage-slack/summary")

	counter, err := meter.Int64Counter("rss.article.count",
		metric.WithDescription("RSS article messages count"),
	)
	if err != nil {
		log.Println("failed to create counter rss.article.count:", err)
		return
	}

	for channelID, hostMap := range countByHostByChannel {
		channel, ok := channelById[channelID]
		if !ok {
			continue
		}
		sanitizedChannel := strings.ReplaceAll(strings.ReplaceAll(channel.Name, ".", "_"), "-", "_")
		for host, v := range hostMap {
			sanitizedHost := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(host, ".", "_"), "-", "_"), "www_", "")
			opts := metric.WithAttributes(
				attribute.String("host", sanitizedHost),
				attribute.String("channel", sanitizedChannel),
			)
			counter.Add(ctx, int64(v), opts)
		}
	}
}
