# frozen_string_literal: true

require 'slack-ruby-client'
require 'parallel'

Slack.configure do |config|
  config.token = ARGV[0]
end

client = Slack::Web::Client.new

start_message = `タスク実行を開始します\n#{Time.now}`

client.chat_postMessage(channel: ARGV[1], text: start_message)

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
