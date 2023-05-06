package main

import (
	"testing"

	"github.com/slack-go/slack"
)

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
		client := slack.New("")
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
			want: "",
		},
	}
	for _, tt := range tests {
		client := slack.New("")
		t.Run(tt.name, func(t *testing.T) {
			got := SlackClient{client}.postStartMessage()
			if got != tt.want {
				t.Errorf("add() = %v, want %v", got, tt.want)
			}
		})
	}
}
