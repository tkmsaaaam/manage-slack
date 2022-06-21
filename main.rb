require 'slack-ruby-client'
require 'parallel'

Slack.configure do |config|
  config.token = ARGV[0]
end

client = Slack::Web::Client.new

channels = client.conversations_list().channels

THREE_DAYS_BEFORE = 60*60*24*3

channels.each do |c|
  res = client.conversations_history(channel: c.id, lgimit: 100, latest: (Time.now - THREE_DAYS_BEFORE).to_i).messages
  until res.size.zero? do
    Parallel.each(res, in_processes: 20) do |r|
      begin
        puts client.chat_delete(channel: c.id, ts: r.ts)
      rescue => e
        puts "Error #{e}"
        sleep(1)
      end
    end
    res = client.conversations_history(channel: c.id, limit: 100, latest: (Time.now - THREE_DAYS_BEFORE).to_i).messages
  end
  puts c.id
end
