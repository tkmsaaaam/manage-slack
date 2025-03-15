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
				t.Errorf("len(getConversationsForUser) = \n%v, want \n%v", got, tt.want.channels)
			}
			gotPrint := strings.TrimRight(buf.String(), "\n")
			if gotPrint != tt.want.print {
				t.Errorf("getConversationsForUser() print = \n%v, want \n%v", gotPrint, tt.want.print)
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
		countBySiteByChannel map[string]map[string]int
		countByHost          map[string]int
		countByChannel       map[string]int
		err                  string
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
			want:   want{countBySiteByChannel: map[string]map[string]int{"ABCDEF12345": {}}, countByHost: map[string]int{}, countByChannel: map[string]int{"ABCDEF12345": 1}},
		},
		{
			name:   "aMessageWithLink",
			args:   args{conversations: []slack.Channel{{GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, now: now, yesterDay: yesterDay},
			apiRes: "testdata/conversationsHistory/messageWithLink.json",
			want:   want{countBySiteByChannel: map[string]map[string]int{"ABCDEF12345": {"bot-user-name": 1}}, countByHost: map[string]int{"example.com": 1}, countByChannel: map[string]int{"ABCDEF12345": 1}},
		},
		{
			name:   "aMessageWithInvalidLink",
			args:   args{conversations: []slack.Channel{{GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, now: now, yesterDay: yesterDay},
			apiRes: "testdata/conversationsHistory/messageWithInvalidLink.json",
			want:   want{countBySiteByChannel: map[string]map[string]int{"ABCDEF12345": {"ABCDEF123": 1}}, countByHost: map[string]int{}, countByChannel: map[string]int{"ABCDEF12345": 1}},
		},
		{
			name:   "twoMessageInDefferentChannel",
			args:   args{conversations: []slack.Channel{{GroupConversation: slack.GroupConversation{Name: "channelNameA", Conversation: slack.Conversation{ID: "ABCDEF01234"}}}, {GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, now: now, yesterDay: yesterDay},
			apiRes: "testdata/conversationsHistory/aMessage.json",
			want:   want{countBySiteByChannel: map[string]map[string]int{"ABCDEF12345": {}, "ABCDEF01234": {}}, countByHost: map[string]int{}, countByChannel: map[string]int{"ABCDEF01234": 1, "ABCDEF12345": 1}},
		},
		{
			name:   "twoMessageInDefferentChannelWithError",
			args:   args{conversations: []slack.Channel{{GroupConversation: slack.GroupConversation{Name: "channelNameA", Conversation: slack.Conversation{ID: "ABCDEF01234"}}}, {GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, now: now, yesterDay: yesterDay},
			apiRes: "testdata/conversationsHistory/error.json",
			want:   want{countByHost: map[string]int{}, countByChannel: map[string]int{}, err: "can not get history channelID: ABCDEF01234 channel_not_found\ncan not get history channelID: ABCDEF12345 channel_not_found"},
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
			actualCountBySiteByChannel, actualCountByHost, actualCountByChannel := c.makeResult(tt.args.conversations)
			if len(actualCountByChannel) != len(tt.want.countByChannel) {
				t.Errorf("len(createChannels().countByChannel) = \n%v, want \n%v", actualCountByChannel, tt.want.countByChannel)
			}
			if len(actualCountByChannel) > 0 {
				for k, v := range tt.want.countByChannel {
					if v != tt.want.countByChannel[k] {
						t.Errorf("createChannels() countByChannel = \n%v, want \n%v", actualCountByChannel[k], v)
					}

				}
			}
			if len(actualCountByHost) != len(tt.want.countByHost) {
				t.Errorf("len(createChannels().countByHost) = \n%v, want \n%v", actualCountByHost, tt.want.countByHost)
			}
			if len(actualCountByHost) > 0 {
				for k, v := range tt.want.countByHost {
					if v != actualCountByHost[k] {
						t.Errorf("createChannels() countByHost %v = \n%v (%v), want \n%v (%v)", k, actualCountByHost[k], actualCountByHost, v, tt.want.countByHost)
					}

				}
			}
			if len(actualCountBySiteByChannel) != len(tt.want.countBySiteByChannel) {
				t.Errorf("len(createChannels().countBySiteByChannel) = %v, want %v", actualCountBySiteByChannel, tt.want.countBySiteByChannel)
			}
			if len(actualCountBySiteByChannel) > 0 {
				for k, v := range tt.want.countBySiteByChannel {
					if len(v) != len(actualCountBySiteByChannel[k]) {
						t.Errorf("len(createChannels() countBySiteByChannel) %v = \n%v (%v), want \n%v (%v)", k, actualCountBySiteByChannel[k], actualCountBySiteByChannel, v, tt.want.countBySiteByChannel)
					}
					for kk, vv := range v {
						if vv != actualCountBySiteByChannel[k][kk] {
							t.Errorf("createChannels() countByHost %v = \n%v (%v), want \n%v (%v)", kk, actualCountBySiteByChannel[k][kk], actualCountBySiteByChannel, vv, tt.want.countBySiteByChannel)
						}
					}

				}
			}
			gotPrint := strings.TrimRight(buf.String(), "\n")
			if gotPrint != tt.want.err {
				t.Errorf("createChannels() = \n%v, want \n%v", gotPrint, tt.want.err)
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
				t.Errorf("createMessage() = \n%v, want \n%v", got, tt.want)
			}
		})
	}
}
