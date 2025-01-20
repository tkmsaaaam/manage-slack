package main

import (
	"bytes"
	"embed"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
)

//go:embed testdata
var testdata embed.FS

func TestGetConversationsForUser(t *testing.T) {
	type want struct {
		channels []slack.Channel
		print    string
	}
	tests := []struct {
		name   string
		apiRes string
		want   want
	}{
		{
			name:   "channelIsNil",
			apiRes: "testdata/usersConversations/channelIsNil.json",
			want:   want{channels: []slack.Channel{}, print: ""},
		},
		{
			name:   "channelIsNotNil",
			apiRes: "testdata/usersConversations/channelIsNotNil.json",
			want:   want{channels: []slack.Channel{{}}, print: ""},
		},
		{
			name:   "usersConversationsError",
			apiRes: "testdata/usersConversations/error.json",
			want:   want{channels: []slack.Channel{}, print: "can not get channels: invalid_auth"},
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/users.conversations", func(w http.ResponseWriter, _ *http.Request) {
				res, _ := testdata.ReadFile(tt.apiRes)
				w.Write(res)
			})
		})
		ts.Start()
		client := slack.New("testToken", slack.OptionAPIURL(ts.GetAPIURL()))
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			var buf bytes.Buffer
			log.SetOutput(&buf)
			defaultFlags := log.Flags()
			log.SetFlags(0)
			defer func() {
				log.SetOutput(os.Stderr)
				log.SetFlags(defaultFlags)
				buf.Reset()
			}()

			c := &config{userClient: client}
			got := c.getConversationsForUser()

			if len(got) != len(tt.want.channels) {
				t.Errorf("add() = %v, want %v", got, tt.want.channels)
			}
			gotPrint := strings.TrimRight(buf.String(), "\n")
			if gotPrint != tt.want.print {
				t.Errorf("add() = %v, want %v", gotPrint, tt.want.print)
			}
		})
	}
}

func TestMakeResult(t *testing.T) {
	type args struct {
		conversations []slack.Channel
		now           time.Time
		yesterDay     time.Time
	}
	type want struct {
		mapByChannel map[string]int
		count        int
		err          string
	}
	now := time.Now()
	yesterDay := now.AddDate(0, 0, -1)
	tests := []struct {
		name   string
		args   args
		apiRes string
		want   want
	}{
		{
			name:   "aMessage",
			args:   args{conversations: []slack.Channel{{GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, now: now, yesterDay: yesterDay},
			apiRes: "testdata/conversationsHistory/aMessage.json",
			want:   want{mapByChannel: map[string]int{"ABCDEF12345": 1}},
		},
		{
			name:   "twoMessageInDefferentChannel",
			args:   args{conversations: []slack.Channel{{GroupConversation: slack.GroupConversation{Name: "channelNameA", Conversation: slack.Conversation{ID: "ABCDEF01234"}}}, {GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, now: now, yesterDay: yesterDay},
			apiRes: "testdata/conversationsHistory/aMessage.json",
			want:   want{mapByChannel: map[string]int{"ABCDEF01234": 1, "ABCDEF12345": 1}},
		},
		{
			name:   "twoMessageInDefferentChannelWithError",
			args:   args{conversations: []slack.Channel{{GroupConversation: slack.GroupConversation{Name: "channelNameA", Conversation: slack.Conversation{ID: "ABCDEF01234"}}}, {GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, now: now, yesterDay: yesterDay},
			apiRes: "testdata/conversationsHistory/error.json",
			want:   want{mapByChannel: map[string]int{}, err: "can not get history channelID: ABCDEF01234, channel_not_found\ncan not get history channelID: ABCDEF12345, channel_not_found"},
		},
	}

	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/conversations.history", func(w http.ResponseWriter, _ *http.Request) {
				res, _ := testdata.ReadFile(tt.apiRes)
				w.Write(res)
			})
		})
		ts.Start()
		client := slack.New("testToken", slack.OptionAPIURL(ts.GetAPIURL()))
		t.Run(tt.name, func(t *testing.T) {

			var buf bytes.Buffer
			log.SetOutput(&buf)
			defaultFlags := log.Flags()
			log.SetFlags(0)
			defer func() {
				log.SetOutput(os.Stderr)
				log.SetFlags(defaultFlags)
				buf.Reset()
			}()

			c := &config{userClient: client}
			_, _, actual := c.makeResult(tt.args.conversations)
			if len(actual) != len(tt.want.mapByChannel) {
				t.Errorf("createChannels() = %v, want %v", actual, tt.want.mapByChannel)
			}
			if len(actual) > 0 {
				for k, v := range actual {
					if v != tt.want.mapByChannel[k] {
						t.Errorf("createChannels() = %v, want %v", v, tt.want.mapByChannel[k])
					}

				}
			}
			gotPrint := strings.TrimRight(buf.String(), "\n")
			if gotPrint != tt.want.err {
				t.Errorf("createChannels() = %v, want %v", gotPrint, tt.want.err)
			}
		})
	}
}

func TestCreateMessage(t *testing.T) {
	type args struct {
		yesterDay          time.Time
		mapBySiteByChannel map[string]map[string]int
		mapByChannel       map[string]int
		channelMap         map[string]slack.Channel
	}

	aDay := time.Date(2023, 1, 1, 0, 0, 0, 0, time.Now().Location())

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "channelsIsNil",
			args: args{yesterDay: aDay},
			want: "2023-01-01\nSunday\n0\n",
		},
		{
			name: "channelsIsZero",
			args: args{yesterDay: aDay, channelMap: map[string]slack.Channel{"ABCDEF12345": {GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, mapByChannel: map[string]int{}},
			want: "2023-01-01\nSunday\n0\n",
		},
		{
			name: "channelsIsPresent",
			args: args{yesterDay: aDay, mapBySiteByChannel: map[string]map[string]int{"ABCDEF12345": {"SiteB": 2, "SiteA": 1}}, channelMap: map[string]slack.Channel{"ABCDEF12345": {GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, mapByChannel: map[string]int{"ABCDEF12345": 1}},
			want: "2023-01-01\nSunday\n1\n\n<#ABCDEF12345>\nSiteB : 2\nSiteA : 1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &config{yesterDay: tt.args.yesterDay}
			got := c.createMessage(tt.args.mapBySiteByChannel, tt.args.mapByChannel, tt.args.channelMap)
			if got != tt.want {
				t.Errorf("add() = \n%v, want \n%v", got, tt.want)
			}
		})
	}
}
