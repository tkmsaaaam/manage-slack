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

func TestGetChannels(t *testing.T) {
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

func TestPostStartMessage(t *testing.T) {
	type want struct {
		ts    string
		print string
	}

	tests := []struct {
		name   string
		apiRes string
		want   want
	}{
		{
			name:   "PostStartMessageOk",
			apiRes: "testdata/chatPostMessage/ok.json",
			want:   want{ts: "1503435956.000247", print: ""},
		},
		{
			name:   "PostStartMessageError",
			apiRes: "testdata/chatPostMessage/error.json",
			want:   want{ts: "", print: "too_many_attachments"},
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/chat.postMessage", func(w http.ResponseWriter, _ *http.Request) {
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
		start        time.Time
		ts           string
		messageCount int
		fileCount    int
	}

	tests := []struct {
		name   string
		args   args
		apiRes string
		want   string
	}{
		{
			name:   "PostEndMessageOk",
			args:   args{start: time.Now(), ts: "1503435956.000247", messageCount: 1, fileCount: 0},
			apiRes: "testdata/chatPostMessage/ok.json",
			want:   "",
		},
		{
			name:   "PostEndMessageError",
			args:   args{start: time.Now(), ts: "1503435956.000247", messageCount: 1, fileCount: 0},
			apiRes: "testdata/chatPostMessage/error.json",
			want:   "too_many_attachments",
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/chat.postMessage", func(w http.ResponseWriter, _ *http.Request) {
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
			SlackClient{client}.postEndMessage(tt.args.start, tt.args.ts, tt.args.messageCount, tt.args.fileCount)
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

func TestDeleteMessage(t *testing.T) {
	type args struct {
		id string
		ts string
	}

	tests := []struct {
		name   string
		args   args
		apiRes string
		want   string
	}{
		{
			name:   "ChatDeleteOk",
			args:   args{id: "ABCDEF123", ts: "1503435956.000247"},
			apiRes: "testdata/chatDelete/ok.json",
			want:   "",
		},
		{
			name:   "ChatDeleteError",
			args:   args{id: "ABCDEF123", ts: "1503435956.000247"},
			apiRes: "testdata/chatDelete/error.json",
			want:   "ABCDEF123:1503435956.000247:cant_delete_message",
		},
		{
			name:   "MessageNotFound",
			args:   args{id: "ABCDEF123", ts: "1503435956.000247"},
			apiRes: "testdata/chatDelete/messageNotFound.json",
			want:   "ABCDEF123:1503435956.000247:message_not_found",
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/chat.delete", func(w http.ResponseWriter, _ *http.Request) {
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

func TestMakeDays(t *testing.T) {
	type want struct {
		res   int
		print string
	}
	tests := []struct {
		name string
		arg  string
		want want
	}{
		{
			name: "CanNotDoAtoi",
			arg:  "a",
			want: want{res: 3, print: "strconv.Atoi: parsing \"a\": invalid syntax"},
		},
		{
			name: "CanDoAtoi",
			arg:  "2",
			want: want{res: 2, print: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			orgStdout := os.Stdout
			defer func() {
				os.Stdout = orgStdout
			}()
			r, w, _ := os.Pipe()
			os.Stdout = w
			got := makeDays(tt.arg)
			if got != tt.want.res {
				t.Errorf("add() = %v, want %v", got, tt.want.res)
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

func TestLoopInAllChannels(t *testing.T) {
	type args struct {
		channels []slack.Channel
		now      time.Time
		days     int
	}
	type want struct {
		count int
		print string
	}

	type apiRes struct {
		conversationsHistory string
		conversationsReplies string
	}

	tests := []struct {
		name   string
		args   args
		apiRes apiRes
		want   want
	}{
		{
			name:   "AMessage",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), days: 3},
			apiRes: apiRes{conversationsHistory: "testdata/conversationsHistory/aMessage.json", conversationsReplies: "testdata/conversationsReplies/messages.json"},
			want:   want{count: 1, print: ""},
		},
		{
			name:   "TwoMessage",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), days: 3},
			apiRes: apiRes{conversationsHistory: "testdata/conversationsHistory/twoMessage.json", conversationsReplies: "testdata/conversationsReplies/messages.json"},
			want:   want{count: 2, print: ""},
		},
		{
			name:   "ConversationsHistoryError",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), days: 3},
			apiRes: apiRes{conversationsHistory: "testdata/conversationsHistory/error.json", conversationsReplies: "testdata/conversationsReplies/messages.json"},
			want:   want{count: 0, print: "channel_not_found"},
		},
		{
			name:   "WithReplyOk",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), days: 3},
			apiRes: apiRes{conversationsHistory: "testdata/conversationsHistory/aMessageWithReply.json", conversationsReplies: "testdata/conversationsReplies/messages.json"},
			want:   want{count: 3, print: ""},
		},
		{
			name:   "WithReplyError",
			args:   args{channels: []slack.Channel{{}}, now: time.Now(), days: 3},
			apiRes: apiRes{conversationsHistory: "testdata/conversationsHistory/aMessageWithReply.json", conversationsReplies: "testdata/conversationsReplies/error.json"},
			want:   want{count: 1, print: "thread_not_found"},
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/conversations.history", func(w http.ResponseWriter, _ *http.Request) {
				res, _ := testdata.ReadFile(tt.apiRes.conversationsHistory)
				w.Write(res)
			})
			c.Handle("/conversations.replies", func(w http.ResponseWriter, _ *http.Request) {
				res, _ := testdata.ReadFile(tt.apiRes.conversationsReplies)
				w.Write(res)
			})
			c.Handle("/chat.delete", func(w http.ResponseWriter, _ *http.Request) {
				res, _ := testdata.ReadFile("testdata/chatDelete/ok.json")
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
			got := SlackClient{client}.loopInAllChannels(tt.args.channels, tt.args.now, tt.args.days)
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

func TestDeleteFiles(t *testing.T) {
	type args struct {
		now  time.Time
		days int
	}
	type want struct {
		count int
		print string
	}

	type apiRes struct {
		files      string
		deleteFile string
	}

	tests := []struct {
		name   string
		args   args
		apiRes apiRes
		want   want
	}{
		{
			name:   "GetFileIsNotOk",
			args:   args{now: time.Now(), days: 3},
			apiRes: apiRes{files: "testdata/files/error.json", deleteFile: "testdata/deleteFile/ok.json"},
			want:   want{count: 0, print: "invalid_auth"},
		},
		{
			name:   "CanDeleteOneFile",
			args:   args{now: time.Now(), days: 3},
			apiRes: apiRes{files: "testdata/files/oneFile.json", deleteFile: "testdata/deleteFile/ok.json"},
			want:   want{count: 1, print: ""},
		},
		{
			name:   "CanDeleteTwoFiles",
			args:   args{now: time.Now(), days: 3},
			apiRes: apiRes{files: "testdata/files/twoFiles.json", deleteFile: "testdata/deleteFile/ok.json"},
			want:   want{count: 2, print: ""},
		},
		{
			name:   "CanNotDeleteTwoFiles",
			args:   args{now: time.Now(), days: 3},
			apiRes: apiRes{files: "testdata/files/twoFiles.json", deleteFile: "testdata/deleteFile/error.json"},
			want:   want{count: 0, print: "invalid_auth\ninvalid_auth"},
		},
	}
	for _, tt := range tests {
		ts := slacktest.NewTestServer(func(c slacktest.Customize) {
			c.Handle("/files.list", func(w http.ResponseWriter, _ *http.Request) {
				res, _ := testdata.ReadFile(tt.apiRes.files)
				w.Write(res)
			})
			c.Handle("/files.delete", func(w http.ResponseWriter, _ *http.Request) {
				res, _ := testdata.ReadFile(tt.apiRes.deleteFile)
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
			got := SlackClient{client}.deleteFiles(tt.args.now, tt.args.days)
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
