package main

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestExecQuery(t *testing.T) {
	type args struct {
		variables map[string]interface{}
	}

	tests := []struct {
		name string
		args args
		want []slack.Channel
	}{
		{
			name: "aaa",
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
