require 'slack-ruby-client'
require 'parallel'

Slack.configure do |config|
  config.token = ARGV[0]
end

client = Slack::Web::Client.new

channels = client.conversations_list().channels

err_count = 0
total_count = 0

channels.each do |c|
  res = client.conversations_history(channel: c.id, limit: 100, latest: (Time.now - (60*60*24*3)).to_i).messages
  count = 0
  until res.size.zero? do
    Parallel.each(res, in_processes: 20) do |r|
      begin
        puts client.chat_delete(channel: c.id, ts: r.ts)
        count += 1
      rescue => e
        puts "Error #{e}"
        sleep(1)
        err_count += 1
      end
    end
    res = client.conversations_history(channel: c.id, limit: 100, latest: (Time.now - (60*60*24*3)).to_i).messages
  end
  total_count += count
  puts "#{c.id}:#{count}"
end

puts "Deleted #{total_count} messages.\nError occurred #{err_count} times."
