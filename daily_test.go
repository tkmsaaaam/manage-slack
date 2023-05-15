//go:build ignore

package main

import (
	"github.com/slack-go/slack"
	"testing"
)

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
