package main

import (
	_ "embed"
	"net/http"
	"testing"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
)

//go:embed testdata/users.conversations.json
var usersConversations []byte

func TestGetChannels(t *testing.T) {
	type args struct {
		variables map[string]interface{}
	}

	tests := []struct {
		name   string
		args   args
		apiRes []byte
		want   []slack.Channel
	}{
		{
			name:   "channelIsNil",
			args:   args{},
			apiRes: usersConversations,
			want:   []slack.Channel{},
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/users.conversations", func(w http.ResponseWriter, _ *http.Request) {
				w.Write(tt.apiRes)
			})
		})
		ts.Start()
		client := slack.New("testToken", slack.OptionAPIURL(ts.GetAPIURL()))
		t.Run(tt.name, func(t *testing.T) {
			got := SlackClient{client}.getChannels()
			if len(got) != len(tt.want) {
				t.Errorf("add() = %v, want %v", got, tt.want)
			}
		})
	}
}

//go:embed testdata/chat.postMessage.json
var chatPostMessage []byte

func TestPostStartMessage(t *testing.T) {
	type args struct {
		variables map[string]interface{}
	}

	tests := []struct {
		name   string
		args   args
		apiRes []byte
		want   string
	}{
		{
			name:   "PostStartMessage",
			args:   args{},
			apiRes: chatPostMessage,
			want:   "1503435956.000247",
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/chat.postMessage", func(w http.ResponseWriter, _ *http.Request) {
				w.Write(tt.apiRes)
			})
		})
		ts.Start()
		client := slack.New("testToken", slack.OptionAPIURL(ts.GetAPIURL()))
		t.Run(tt.name, func(t *testing.T) {
			got := SlackClient{client}.postStartMessage()
			if got != tt.want {
				t.Errorf("add() = %v, want %v", got, tt.want)
			}
		})
	}
}
