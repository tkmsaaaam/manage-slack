package main

import (
	"context"
	"fmt"
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

type SlackClient struct {
	*slack.Client
}

func (client *SlackClient) getChannels() ([]slack.Channel, error) {
	channels, _, err := client.GetConversationsForUser(&slack.GetConversationsForUserParameters{})
	if err != nil {
		return nil, fmt.Errorf("can not get channels: %w", err)
	}
	return channels, nil
}

func (client *SlackClient) postStartMessage() string {
	_, ts, err := client.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText("タスク実行を開始します", true))
	if err != nil {
		log.Println("Can not post start message:", err)
	}
	return ts
}

func (client *SlackClient) postEndMessage(duration time.Duration, ts string, messageCount, fileCount int) {
	avg := float64(messageCount) / duration.Seconds()
	message := "タスク実行を終了します\n" + duration.String() + "\n" + "message count: " + strconv.FormatInt(int64(messageCount), 10) + "\n" + "avg: " + strconv.FormatFloat(avg, 'f', -1, 64) + "/s" + "\n" + "file count: " + strconv.FormatInt(int64(fileCount), 10)
	_, _, err := client.PostMessage(os.Getenv("SLACK_CHANNEL_ID"), slack.MsgOptionText(message, true), slack.MsgOptionTS(ts), slack.MsgOptionBroadcast())
	if err != nil {
		log.Println("End message can not post:", err)
	}
}

func (client *SlackClient) deleteMessage(id, ts string) {
	_, _, err := client.DeleteMessage(id, ts)
	if err != nil {
		log.Println("Can not delete message:", id, ":", ts, ":", err)
		if err.Error() != "message_not_found" {
			recover()
		}
	}
}

func makeDays(daysStr string) int {
	days, err := strconv.Atoi(daysStr)
	if err != nil {
		log.Println("env DAYS is invalid:", err)
		const DAFAULT_DAYS = 3
		return DAFAULT_DAYS
	}
	return days
}

func (client *SlackClient) loopInAllChannels(channels []slack.Channel, now time.Time, days int) map[string]int {
	countByChannel := map[string]int{}
	for _, channel := range channels {
		id := channel.ID
		latest := strconv.FormatInt(now.AddDate(0, 0, -days).Unix(), 10)
		params := slack.GetConversationHistoryParameters{ChannelID: id, Limit: 1000, Latest: latest}
		res, err := client.GetConversationHistory(&params)
		if err != nil {
			log.Println("Can not get history:", err)
			continue
		}
		count := 0
		for _, message := range res.Messages {
			if len(message.Reactions) > 0 {
				continue
			}
			count++
			if message.ReplyCount != 0 {
				repliesParams := slack.GetConversationRepliesParameters{ChannelID: id, Timestamp: message.Msg.Timestamp}
				replies, _, _, err := client.GetConversationReplies(&repliesParams)
				if err != nil {
					log.Println("Can not get replies:", err)
				} else {
					for _, reply := range replies {
						count++
						client.deleteMessage(id, reply.Msg.Timestamp)
					}
				}
			}
			client.deleteMessage(id, message.Msg.Timestamp)
		}
		countByChannel[id] = count
	}
	return countByChannel
}

func (client *SlackClient) deleteFiles(now time.Time, days int) int {
	latest := now.AddDate(0, 0, -days).Unix()
	params := slack.GetFilesParameters{TimestampTo: slack.JSONTime(latest)}
	res, _, err := client.GetFiles(params)
	count := 0
	if err != nil {
		log.Println("Can not get file:", err)
		return count
	}
	for _, file := range res {
		id := file.ID
		err := client.DeleteFile(id)
		if err != nil {
			log.Println("Can not delete file:", err)
			continue
		}
		count++
	}
	return count
}

func main() {
	botClient := &SlackClient{slack.New(os.Getenv("SLACK_BOT_TOKEN"))}
	userClient := &SlackClient{slack.New(os.Getenv("SLACK_USER_TOKEN"))}
	start := time.Now()
	ts := botClient.postStartMessage()
	channels, err := userClient.getChannels()
	if err != nil {
		log.Println("Can not get channels", err)
		return
	}
	daysStr := os.Getenv("DAYS")
	days := makeDays(daysStr)
	countByChannel := userClient.loopInAllChannels(channels, start, days)
	messageCount := 0
	for _, c := range countByChannel {
		messageCount += c
	}
	channelById := map[string]slack.Channel{}
	for _, ch := range channels {
		channelById[ch.ID] = ch
	}
	fileCount := botClient.deleteFiles(start, days)
	duration := time.Since(start)
	botClient.postEndMessage(duration, ts, messageCount, fileCount)
	sendMetrics(countByChannel, channelById, fileCount, duration)
}

func sendMetrics(countByChannel map[string]int, channelById map[string]slack.Channel, fileCount int, duration time.Duration) {
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
			attribute.String("service.name", "manage-slack/remover"),
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

	meter := provider.Meter("github.com/tkmsaaaam/manage-slack/remover")

	deletedMessagesCounter, err := meter.Int64Counter("slack_deleted_messages",
		metric.WithDescription("Number of deleted messages"),
	)
	if err != nil {
		log.Println("failed to create deleted messages counter:", err)
	}

	deletedFilesCounter, err := meter.Int64Counter("slack_deleted_files",
		metric.WithDescription("Number of deleted files"),
	)
	if err != nil {
		log.Println("failed to create deleted files counter:", err)
	}

	removerDuration, err := meter.Float64Histogram("slack_remover_duration",
		metric.WithDescription("Duration of the remover run in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		log.Println("failed to create duration histogram:", err)
	}

	if deletedMessagesCounter != nil {
		for channelID, count := range countByChannel {
			channel, ok := channelById[channelID]
			if !ok {
				continue
			}
			sanitizedChannel := strings.ReplaceAll(strings.ReplaceAll(channel.Name, ".", "_"), "-", "_")
			opts := metric.WithAttributes(
				attribute.String("channel", sanitizedChannel),
			)
			deletedMessagesCounter.Add(ctx, int64(count), opts)
		}
	}
	if deletedFilesCounter != nil {
		deletedFilesCounter.Add(ctx, int64(fileCount))
	}
	if removerDuration != nil {
		removerDuration.Record(ctx, duration.Seconds())
	}
}
