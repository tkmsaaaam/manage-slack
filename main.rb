require 'slack-ruby-client'

Slack.configure do |config|
  config.token = ARGV[0]
end

client = Slack::Web::Client.new

channels = client.conversations_list().channels

err_count = 0

begin
  channels.each do |c|
    res = client.conversations_history(channel: c.id, has_more: true).messages
    count = 0
    until res.size.zero? do
      res.each do |a|
        puts client.chat_delete(channel: c.id, ts: a.ts) if a.ts.to_i < (Time.now - (60*60*24*3)).to_i
      end
      res = client.conversations_history(channel: c.id, has_more: true).messages
    end
    puts "#{c.name}:#{count}"
  end
rescue => e
  puts "Error #{e}"
  sleep(1)
  err_count += 1
  retry if err_count < 10
end
