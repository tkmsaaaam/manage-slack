package main

import (
	"bytes"
	_ "embed"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
)

//go:embed testdata/usersConversations/channelIsNil.json
var channelIsNil []byte

//go:embed testdata/usersConversations/channelIsNotNil.json
var channelIsNotNil []byte

//go:embed testdata/usersConversations/error.json
var usersConversationsError []byte

func TestGetChannels(t *testing.T) {
	type want struct {
		channels []slack.Channel
		print    string
	}

	tests := []struct {
		name   string
		apiRes []byte
		want   want
	}{
		{
			name:   "channelIsNil",
			apiRes: channelIsNil,
			want:   want{channels: []slack.Channel{}, print: ""},
		},
		{
			name:   "channelIsNotNil",
			apiRes: channelIsNotNil,
			want:   want{channels: []slack.Channel{{}}, print: ""},
		},
		{
			name:   "channelIsNotNil",
			apiRes: usersConversationsError,
			want:   want{channels: []slack.Channel{}, print: "invalid_auth"},
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
			t.Helper()

			orgStdout := os.Stdout
			defer func() {
				os.Stdout = orgStdout
			}()
			r, w, _ := os.Pipe()
			os.Stdout = w
			gotChannels := SlackClient{client}.getChannels()
			if len(gotChannels) != len(tt.want.channels) {
				t.Errorf("add() = %v, want %v", gotChannels, tt.want.channels)
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

//go:embed testdata/chatPostMessage/ok.json
var chatPostMessageOk []byte

//go:embed testdata/chatPostMessage/error.json
var chatPostMessageError []byte

func TestPostStartMessage(t *testing.T) {
	type want struct {
		ts    string
		print string
	}

	tests := []struct {
		name   string
		apiRes []byte
		want   want
	}{
		{
			name:   "PostStartMessageOk",
			apiRes: chatPostMessageOk,
			want:   want{ts: "1503435956.000247", print: ""},
		},
		{
			name:   "PostStartMessageError",
			apiRes: chatPostMessageError,
			want:   want{ts: "", print: "too_many_attachments"},
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
			t.Helper()

			orgStdout := os.Stdout
			defer func() {
				os.Stdout = orgStdout
			}()
			r, w, _ := os.Pipe()
			os.Stdout = w
			got := SlackClient{client}.postStartMessage()
			if got != tt.want.ts {
				t.Errorf("add() = %v, want %v", got, tt.want.ts)
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

func TestPostEndMessage(t *testing.T) {
	type args struct {
		start time.Time
		ts    string
		count int
	}

	tests := []struct {
		name   string
		args   args
		apiRes []byte
		want   string
	}{
		{
			name:   "PostEndMessageOk",
			args:   args{start: time.Now(), ts: "1503435956.000247", count: 1},
			apiRes: chatPostMessageOk,
			want:   "",
		},
		{
			name:   "PostEndMessageError",
			args:   args{start: time.Now(), ts: "1503435956.000247", count: 1},
			apiRes: chatPostMessageError,
			want:   "too_many_attachments",
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
			t.Helper()

			orgStdout := os.Stdout
			defer func() {
				os.Stdout = orgStdout
			}()
			r, w, _ := os.Pipe()
			os.Stdout = w
			SlackClient{client}.postEndMessage(tt.args.start, tt.args.ts, tt.args.count)
			w.Close()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				t.Fatalf("failed to read buf: %v", err)
			}
			gotPrint := strings.TrimRight(buf.String(), "\n")
			if gotPrint != tt.want {
				t.Errorf("add() = %v, want %v", gotPrint, tt.want)
			}
		})
	}
}

//go:embed testdata/chatDelete/ok.json
var chatDeleteOk []byte

//go:embed testdata/chatDelete/error.json
var chatDeleteError []byte

//go:embed testdata/chatDelete/messageNotFound.json
var messageNotFound []byte

func TestDeleteMessage(t *testing.T) {
	type args struct {
		id string
		ts string
	}

	tests := []struct {
		name   string
		args   args
		apiRes []byte
		want   string
	}{
		{
			name:   "ChatDeleteOk",
			args:   args{id: "ABCDEF123", ts: "1503435956.000247"},
			apiRes: chatDeleteOk,
			want:   "",
		},
		{
			name:   "ChatDeleteError",
			args:   args{id: "ABCDEF123", ts: "1503435956.000247"},
			apiRes: chatDeleteError,
			want:   "ABCDEF123:1503435956.000247:cant_delete_message",
		},
		{
			name:   "MessageNotFound",
			args:   args{id: "ABCDEF123", ts: "1503435956.000247"},
			apiRes: messageNotFound,
			want:   "ABCDEF123:1503435956.000247:message_not_found",
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/chat.delete", func(w http.ResponseWriter, _ *http.Request) {
				w.Write(tt.apiRes)
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
			SlackClient{client}.deleteMessage(tt.args.id, tt.args.ts)
			w.Close()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				t.Fatalf("failed to read buf: %v", err)
			}
			gotPrint := strings.TrimRight(buf.String(), "\n")
			if gotPrint != tt.want {
				t.Errorf("add() = %v, want %v", gotPrint, tt.want)
			}
		})
	}
}

//go:embed testdata/conversationsHistory/aMessage.json
var aMessage []byte

//go:embed testdata/conversationsHistory/aMessageWithReply.json
var aMessageWithReply []byte

//go:embed testdata/conversationsHistory/twoMessage.json
var twoMessage []byte

//go:embed testdata/conversationsHistory/error.json
var conversationsHistoryError []byte

//go:embed testdata/conversationsReplies/messages.json
var conversationsRepliesMessages []byte

//go:embed testdata/conversationsReplies/error.json
var conversationsRepliesError []byte

func TestLoopInAllChannels(t *testing.T) {
	type args struct {
		channels []slack.Channel
		now      time.Time
		daysStr  string
	}
	type want struct {
		count int
		print string
	}

	type apiRes struct {
		conversationsHistory []byte
		conversationsReplies []byte
	}

	tests := []struct {
		name   string
		args   args
		apiRes apiRes
		want   want
	}{
		{
			name:   "InvalidDaysStr",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), daysStr: "A"},
			apiRes: apiRes{conversationsHistory: aMessage, conversationsReplies: conversationsRepliesMessages},
			want:   want{count: 1, print: "strconv.Atoi: parsing \"A\": invalid syntax"},
		},
		{
			name:   "AMessage",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), daysStr: "3"},
			apiRes: apiRes{conversationsHistory: aMessage, conversationsReplies: conversationsRepliesMessages},
			want:   want{count: 1, print: ""},
		},
		{
			name:   "TwoMessage",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), daysStr: "3"},
			apiRes: apiRes{conversationsHistory: twoMessage, conversationsReplies: conversationsRepliesMessages},
			want:   want{count: 2, print: ""},
		},
		{
			name:   "ConversationsHistoryError",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), daysStr: "3"},
			apiRes: apiRes{conversationsHistory: conversationsHistoryError, conversationsReplies: conversationsRepliesMessages},
			want:   want{count: 0, print: "channel_not_found"},
		},
		{
			name:   "WithReplyOk",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), daysStr: "3"},
			apiRes: apiRes{conversationsHistory: aMessageWithReply, conversationsReplies: conversationsRepliesMessages},
			want:   want{count: 3, print: ""},
		},
		{
			name:   "WithReplyError",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), daysStr: "3"},
			apiRes: apiRes{conversationsHistory: aMessageWithReply, conversationsReplies: conversationsRepliesError},
			want:   want{count: 1, print: "thread_not_found"},
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/conversations.history", func(w http.ResponseWriter, _ *http.Request) {
				w.Write(tt.apiRes.conversationsHistory)
			})
			c.Handle("/conversations.replies", func(w http.ResponseWriter, _ *http.Request) {
				w.Write(tt.apiRes.conversationsReplies)
			})
			c.Handle("/chat.delete", func(w http.ResponseWriter, _ *http.Request) {
				w.Write(chatDeleteOk)
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
			got := SlackClient{client}.loopInAllChannels(tt.args.channels, tt.args.now, tt.args.daysStr)
			if got != tt.want.count {
				t.Errorf("add() = %v, want %v", got, tt.want.count)
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
