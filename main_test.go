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

//go:embed testdata/chat.postMessage.json
var chatPostMessage []byte

func TestGetChannels(t *testing.T) {
	type args struct {
		variables map[string]interface{}
	}

	tests := []struct {
		name string
		args args
		want []slack.Channel
	}{
		{
			name: "channelIsNil",
			args: args{},
			want: []slack.Channel{},
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/users.conversations", func(w http.ResponseWriter, r *http.Request) {
				w.Write(usersConversations)
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

func TestPostStartMessage(t *testing.T) {
	type args struct {
		variables map[string]interface{}
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "PostStartMessage",
			args: args{},
			want: "1503435956.000247",
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
				w.Write(chatPostMessage)
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
