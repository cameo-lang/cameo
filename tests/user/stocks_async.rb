# Downloads daily stock prices from Yahoo Finance into stocks.csv in the
# Data folder, as stocks.rb does, but fetches several tickers at once with
# async: a ticker's parts are fetched one after the other, and up to
# parallel tickers are fetched at the same time.
#
# INPUT is a Hash; each key is optional:
#
#   tickers   the symbols, the 20 US large caps of TICKERS when not given
#   from      the first day, YYYY-MM-DD; 2016-10-01 when not given
#   name      the file, stocks.csv when not given
#   append    true to add the rows to the file instead of replacing it
#   parallel  how many tickers to fetch at once, 5 when not given
#
# A script stops after five minutes. Fetching at once makes a run shorter,
# but Yahoo answers many requests at once with fewer answers, so keep
# parallel small; a ticker Yahoo keeps refusing fails the run.
#
# The file is what stocks.rb writes: a row per ticker per trading day,
# ticker by ticker in the order of tickers, each by date: date, ticker,
# open, high, low, close, adj_close, volume. Prices are in the ticker's
# currency, adjusted for splits; adj_close is also adjusted for dividends.
# A day Yahoo has no price for is left out. The workers return their rows,
# and the caller writes them in the order of tickers, a ticker at a time,
# as each is in.
#
# The prices come from Yahoo's chart API, through web_fetch. Yahoo Finance
# data is for personal, informational use under Yahoo's terms; it is not
# licensed for redistribution or commercial use.
#
# web_fetch returns at most 100 KB of a page, so the script fetches two
# years of a ticker at a time. It parses Yahoo's answer with String#index
# and #split, so that it runs on cameo, which has no parser for it.
# cameo/runner gives Ruby cameo's async and await, so the script runs on
# both.

require "cameo/runner"

TICKERS = %w[AAPL MSFT AMZN GOOGL NVDA META TSLA JPM V JNJ PG XOM WMT DIS KO PFE BA NFLX INTC HD]
COLUMNS = %w[date ticker open high low close adj_close volume]
HOSTS = %w[query1.finance.yahoo.com query2.finance.yahoo.com]
DAY = 86_400
PART = 2 * 365 * DAY

# days_of returns the days from 1970-01-01 to the date y-m-d.
def days_of(y, m, d)
  y -= 1 if m <= 2
  era = y / 400
  yoe = y - era * 400
  doy = (153 * (m > 2 ? m - 3 : m + 9) + 2) / 5 + d - 1
  doe = yoe * 365 + yoe / 4 - yoe / 100 + doy
  era * 146_097 + doe - 719_468
end

# date_of returns the date, YYYY-MM-DD in UTC, of the Unix time t.
def date_of(t)
  z = t / DAY + 719_468
  era = z / 146_097
  doe = z - era * 146_097
  yoe = (doe - doe / 1460 + doe / 36_524 - doe / 146_096) / 365
  doy = doe - (365 * yoe + yoe / 4 - yoe / 100)
  mp = (5 * doy + 2) / 153
  d = doy - (153 * mp + 2) / 5 + 1
  m = mp < 10 ? mp + 3 : mp - 9
  y = yoe + era * 400 + (m <= 2 ? 1 : 0)
  format("%04d-%02d-%02d", y, m, d)
end

# list_of returns the values of the array of numbers named key in
# json, as Strings, [] when json has none. An array of objects of that
# name, such as Yahoo's outer adjclose, is skipped.
def list_of(json, key)
  at = json.index("\"#{key}\":[")
  at = json.index("\"#{key}\":[", at + 1) while !at.nil? && json[at + key.size + 4] == "{"
  return [] if at.nil?
  from = at + key.size + 4
  to = json.index("]", from)
  return [] if to.nil? || to == from
  json[from...to].split(",")
end

# number_of returns the number named key in json, nil when json has
# none.
def number_of(json, key)
  at = json.index("\"#{key}\":")
  return nil if at.nil?
  from = at + key.size + 3
  to = from
  to += 1 while to < json.size && "-0123456789.".include?(json[to])
  to == from ? nil : json[from...to].to_i
end

# rows_of returns the CSV rows of ticker in Yahoo's chart data, from the
# day after last on; a row with a null is left out.
def rows_of(json, ticker, last)
  times = list_of(json, "timestamp")
  open = list_of(json, "open")
  high = list_of(json, "high")
  low = list_of(json, "low")
  close = list_of(json, "close")
  adj = list_of(json, "adjclose")
  volume = list_of(json, "volume")
  rows = []
  times.each_with_index do |t, i|
    values = [open[i], high[i], low[i], close[i], adj[i], volume[i]]
    next if values.any? { |v| v.nil? || v == "null" }
    date = date_of(t.to_i)
    next if !last.nil? && date <= last
    prices = values.first(5).map { |v| format("%.4f", v.to_f) }
    rows << ([date, ticker] + prices + [volume[i].to_i.to_s]).join(",")
  end
  rows
end

# json_of returns the chart data in a page web_fetch returned, :none when
# Yahoo has no such ticker, and nil when the page has no chart, as when
# Yahoo asks for fewer requests. web_fetch puts a line or two before the
# data, its status or a title, depending on how it fetched it.
def json_of(page)
  return :none if page.include?("\"result\":null")
  at = page.index("{\"chart\":")
  at.nil? ? nil : page[at, page.size - at]
end

# fetch returns Yahoo's chart data of ticker from the Unix time from up to
# to, nil when Yahoo has no such ticker. A page with no chart, as when
# Yahoo asks for fewer requests, is fetched again from Yahoo's other host,
# up to HOSTS.size * 3 times in all: cameo has no sleep to wait with.
def fetch(ticker, from, to)
  tries = 0
  json = nil
  while json.nil?
    raise "Yahoo returned no prices of #{ticker} after #{tries} tries; run the script again later" if tries == HOSTS.size * 3
    host = HOSTS[tries % HOSTS.size]
    json = json_of(Spicy.tools("web_fetch", url: "https://#{host}/v8/finance/chart/#{ticker}?period1=#{from}&period2=#{to}&interval=1d&events=div,split"))
    tries += 1
  end
  json == :none ? nil : json
end

# A sample of Yahoo's chart data: the parsing runs on it before the first
# tool call, so that a script cameo cannot run fails
# while it can still run on Ruby.
SAMPLE = '{"chart":{"result":[{"meta":{"regularMarketTime":1792612801},"timestamp":[1475501400,1475587800],"indicators":{"quote":[{"open":[28.177499771118164,null],"high":[28.2625,28.5775],"low":[28.07,28.1575],"close":[28.13,28.25],"volume":[86807200,118947200]}],"adjclose":[{"adjclose":[25.7317,25.8415]}]}}]}}'
unless json_of("200 OK, application/json\n\n" + SAMPLE) == SAMPLE && json_of("404 Not Found, application/json\n\n{\"chart\":{\"result\":null}}") == :none &&
       json_of("429 Too Many Requests, text/plain\n\nToo Many Requests").nil? &&
       rows_of(SAMPLE, "AAPL", "2016-10-03").empty? && rows_of(SAMPLE, "AAPL", nil) == ["2016-10-03,AAPL,28.1775,28.2625,28.0700,28.1300,25.7317,86807200"] && number_of(SAMPLE, "regularMarketTime") == 1_792_612_801 && days_of(2016, 10, 3) * DAY + 48_600 == 1_475_501_400
  raise "the script parses Yahoo's chart data wrongly"
end

# Prices fetches tickers on workers, and writes their rows as they come.
class Prices
  include Cameo::Runner

  # rows returns the CSV rows of ticker from the Unix time start on, [] when
  # Yahoo has no such ticker.
  def rows(ticker, start)
    from = start
    now = nil
    last = nil
    all = []
    while now.nil? || from <= now
      json = fetch(ticker, from, from + PART)
      break if json.nil?

      now = number_of(json, "regularMarketTime") || (from + PART)
      part = rows_of(json, ticker, last)
      unless part.empty?
        all += part
        last = part.last.split(",").first
      end
      from += PART
    end
    all
  end

  # download fetches tickers parallel at a time into the file name, and
  # returns the tickers Yahoo has no prices of.
  def download(tickers, start, name, parallel)
    missing = []
    total = 0
    tickers.each_slice(parallel) do |group|
      promises = group.map { |ticker| async self, :rows, ticker, start }
      group.each_with_index do |ticker, i|
        rows = await promises[i]
        Spicy.tools("spicy-data", name: name, text: rows.join("\n") + "\n", append: true) unless rows.empty?
        missing << ticker if rows.empty?
        total += rows.size
        puts "#{ticker}: #{rows.size} days#{rows.empty? ? "" : ", to #{rows.last.split(",").first}"}"
      end
    end
    puts
    puts "#{total} rows of #{tickers.size - missing.size} tickers in #{name}"
    missing
  end
end

input = INPUT || {}
tickers = (input["tickers"] || TICKERS).map { |t| t.to_s.strip.upcase }
first = (input["from"] || "2016-10-01").to_s.split("-").map(&:to_i)
raise "from is a date, YYYY-MM-DD" unless first.size == 3
start = days_of(first[0], first[1], first[2]) * DAY
name = (input["name"] || "stocks.csv").to_s
parallel = (input["parallel"] || 5).to_i
raise "parallel is a number of tickers, 1 or more" if parallel < 1

Spicy.tools("spicy-data", name: name, text: COLUMNS.join(",") + "\n") unless input["append"]
missing = Prices.new.download(tickers, start, name, parallel)
puts "Yahoo has no prices of #{missing.join(", ")}" unless missing.empty?
