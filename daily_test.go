//go:build ignore

package main

import (
	"bytes"
	"embed"
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
			want:   want{channels: []slack.Channel{}, print: "invalid_auth"},
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

			orgStdout := os.Stdout
			defer func() {
				os.Stdout = orgStdout
			}()
			r, w, _ := os.Pipe()
			os.Stdout = w
			got := SlackClient{client}.getConversationsForUser()
			if len(got) != len(tt.want.channels) {
				t.Errorf("add() = %v, want %v", got, tt.want.channels)
			}
			w.Close()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				t.Fatalf("failed to read buf: %v", err)
			}
			gotPrint := strings.TrimRight(buf.String(), "\n")
			if gotPrint != tt.want.print {
				t.Errorf("add() = %v, want %v", gotPrint, tt.want.print)
			}
		})
	}
}

func TestCreateChannels(t *testing.T) {
	type args struct {
		conversations []slack.Channel
		now           time.Time
		yesterDay     time.Time
	}
	type want struct {
		channels []Channel
		count    int
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
			want:   want{channels: []Channel{{name: "channelName", id: "ABCDEF12345", Sites: []Site{{name: "ABCDEF123", count: 1}}}}, count: 1},
		},
		{
			name:   "twoMessageInDefferentChannel",
			args:   args{conversations: []slack.Channel{{GroupConversation: slack.GroupConversation{Name: "channelNameA", Conversation: slack.Conversation{ID: "ABCDEF01234"}}}, {GroupConversation: slack.GroupConversation{Name: "channelName", Conversation: slack.Conversation{ID: "ABCDEF12345"}}}}, now: now, yesterDay: yesterDay},
			apiRes: "testdata/conversationsHistory/aMessage.json",
			want:   want{channels: []Channel{{name: "channelName", id: "ABCDEF12345", Sites: []Site{{name: "ABCDEF123", count: 1}}}, {name: "channelNameA", id: "ABCDEF01234", Sites: []Site{{name: "ABCDEF123", count: 1}}}}, count: 2},
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
			gotChannels, gotCount := createChannels(tt.args.conversations, tt.args.now, tt.args.yesterDay, SlackClient{client})
			if len(gotChannels) != len(tt.want.channels) {
				t.Errorf("add() = %v, want %v", gotChannels, tt.want.channels)
			}
			if len(gotChannels) > 0 && len(gotChannels[0].Sites) != len(tt.want.channels[0].Sites) {
				t.Errorf("add() = %v, want %v", gotChannels, tt.want.channels)
			}
			if len(gotChannels) > 0 && len(gotChannels[0].name) != len(tt.want.channels[0].name) {
				t.Errorf("add() = %v, want %v", gotChannels, tt.want.channels)
			}
			if gotCount != tt.want.count {
				t.Errorf("add() = %v, want %v", gotCount, tt.want.count)
			}
		})
	}
}

func TestCreateMessage(t *testing.T) {
	type args struct {
		yesterDay time.Time
		channels  []Channel
		count     int
	}

	aDay := time.Date(2023, 1, 1, 0, 0, 0, 0, time.Now().Location())

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "channelsIsNil",
			args: args{yesterDay: aDay, channels: []Channel{}, count: 0},
			want: "2023-01-01\nSunday\n0\n",
		},
		{
			name: "channelsIsZero",
			args: args{yesterDay: aDay, channels: []Channel{{name: "A", id: "ABCDEF12345", Sites: []Site{}}}, count: 0},
			want: "2023-01-01\nSunday\n0\n",
		},
		{
			name: "channelsIsPresent",
			args: args{yesterDay: aDay, channels: []Channel{{name: "A", id: "ABCDEF12345", Sites: []Site{{name: "SiteA", count: 1}, {name: "SiteB", count: 2}}}}, count: 3},
			want: "2023-01-01\nSunday\n3\n\n<#ABCDEF12345>\nSiteB : 2\nSiteA : 1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createMessage(tt.args.yesterDay, tt.args.channels, tt.args.count)
			if got != tt.want {
				t.Errorf("add() = \n%v, want \n%v", got, tt.want)
			}
		})
	}
}

func TestAddUser(t *testing.T) {
	type args struct {
		channel *Channel
		message slack.Message
		threads []slack.Message
	}

	tests := []struct {
		name string
		args args
		want *Channel
	}{
		{
			name: "newUser",
			args: args{channel: &Channel{}, message: slack.Message{Msg: slack.Msg{Username: "userName"}}, threads: []slack.Message{}},
			want: &Channel{Sites: []Site{{name: "userName", count: 1}}},
		},
		{
			name: "addCount",
			args: args{channel: &Channel{Sites: []Site{{name: "userName", count: 1}}}, message: slack.Message{Msg: slack.Msg{Username: "userName"}}, threads: []slack.Message{}},
			want: &Channel{Sites: []Site{{name: "userName", count: 2}}},
		},
		{
			name: "newUserNameFromThread",
			args: args{channel: &Channel{}, message: slack.Message{Msg: slack.Msg{ThreadTimestamp: "1503435956.000247"}}, threads: []slack.Message{{Msg: slack.Msg{ThreadTimestamp: "1503435956.000247", Username: "userName"}}}},
			want: &Channel{Sites: []Site{{name: "userName", count: 1}}},
		},
	}
	for _, tt := range tests {
		addUser(tt.args.channel, tt.args.message, tt.args.threads)
		if len(tt.args.channel.Sites) != len(tt.want.Sites) {
			t.Errorf("add() = %v, want %v", tt.args.channel, tt.want)
		}
	}
}

func TestSetName(t *testing.T) {
	type args struct {
		name    string
		message slack.Message
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "userNameIsPresented",
			args: args{name: "default", message: slack.Message{Msg: slack.Msg{Username: "userName"}}},
			want: "userName",
		},
		{
			name: "userNameIsPresentedAndBotNameIsPresented",
			args: args{name: "default", message: slack.Message{Msg: slack.Msg{Username: "userName", BotProfile: &slack.BotProfile{Name: "botName"}}}},
			want: "userName",
		},
		{
			name: "botNameIsPresented",
			args: args{name: "default", message: slack.Message{Msg: slack.Msg{BotProfile: &slack.BotProfile{Name: "botName"}}}},
			want: "botName",
		},
		{
			name: "nilAll",
			args: args{name: "", message: slack.Message{}},
			want: "",
		},
		{
			name: "defaultOnly",
			args: args{name: "default", message: slack.Message{}},
			want: "default",
		},
	}

	for _, tt := range tests {
		got := setName(tt.args.name, tt.args.message)
		if got != tt.want {
			t.Errorf("add() = %v, want %v", got, tt.want)
		}
	}
}
