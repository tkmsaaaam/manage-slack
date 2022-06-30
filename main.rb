# frozen_string_literal: true

require 'slack-ruby-client'
require 'parallel'

client = Slack::Web::Client.new(token: ARGV[0])
client_bot = Slack::Web::Client.new(token: ARGV[2])

START_MESSAGE = "タスク実行を開始します\n#{Time.now}"
END_MESSAGE = "タスク実行を終了します\n#{Time.now}"

thread_ts = client_bot.chat_postMessage(channel: ARGV[1], text: START_MESSAGE).ts

channels = client.conversations_list.channels

THREE_DAYS_BEFORE = 60 * 60 * 24 * 3

channels.each do |c|
  res = client.conversations_history(channel: c.id, lgimit: 100, latest: (Time.now - THREE_DAYS_BEFORE).to_i).messages
  until res.size.zero?
    Parallel.each(res, in_processes: 20) do |r|
      puts client.chat_delete(channel: c.id, ts: r.ts)
    rescue StandardError => e
      puts "Error #{e}"
      sleep(1)
    end
    res = client.conversations_history(channel: c.id, limit: 100, latest: (Time.now - THREE_DAYS_BEFORE).to_i).messages
  end
  puts c.id
end

client_bot.chat_postMessage(channel: ARGV[1], text: END_MESSAGE, thread_ts: thread_ts, reply_broadcast: true)
