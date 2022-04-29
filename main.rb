require 'slack-ruby-client'

Slack.configure do |config|
  config.token = ARGV[0]
end

client = Slack::Web::Client.new

channels = client.conversations_list().channels

err_count = 0

begin
  channels.each do |c|
    res = client.conversations_history(channel: c.id, limit: 100, latest: (Time.now - (60*60*24*3)).to_i).messages
    count = 0
    until res.size.zero? do
      res.each do |a|
        puts client.chat_delete(channel: c.id, ts: a.ts)
        count += 1
      end
      res = client.conversations_history(channel: c.id, limit: 100, latest: (Time.now - (60*60*24*3)).to_i).messages
    end
    puts "#{c.id}:#{count}"
  end
rescue => e
  puts "Error #{e}"
  sleep(1)
  err_count += 1
  retry if err_count < 50
end
