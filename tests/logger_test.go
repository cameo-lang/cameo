package tests

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected lines are Ruby's, with logger 1.6.4; the process id is
// replaced, since the test's own differs each run.

func TestLoggerWritesRubysLines(t *testing.T) {
	out := runScript(t, `require "logger"
log = Logger.new(STDOUT)
log.datetime_format = "%Y"
p log.level, log.debug?, log.info?, log.progname, log.formatter, log.datetime_format, Logger::VERSION, Logger::DEBUG, Logger::UNKNOWN, Logger::SEV_LABEL
log.info("hello")
log.warn { "from block" }
log.add(Logger::ERROR, "added", "prog")
log.error(RuntimeError.new("boom"))
log.debug([1, 2])
log.unknown("u")
log.info(nil)
log.info(:sym)
log.warn(nil) { "blk" }
log.info("p") { "m" }
log.add(nil, "nilsev")
log.add(Logger::WARN, nil, "pg") { "blockm" }
log.add(7, "high")
log.log(Logger::INFO, "log")
log.progname = "app"
log.info("with prog")
log.level = :warn
p log.level, log.info?, log.warn?, log.fatal?
log.info("dropped")
log.warn("kept")
log.level = "error"
p log.level
log.level = Logger::DEBUG
log << "raw\n"
log.formatter = proc { |sev, time, prog, msg| "#{sev}|#{prog}|#{msg.inspect}|#{time.class}\n" }
log.info("custom")
log.info("app2") { "blockmsg" }
log.debug { 42 }
quiet = Logger.new(nil)
quiet.info("nothing")
p quiet.level, quiet.info?
l2 = Logger.new(STDOUT, level: :info, progname: "svc", datetime_format: "%H")
l2.debug("no")
l2.info("yes")
p l2.progname, l2.level, Logger.new(STDOUT, level: Logger::INFO).level, Logger::Error.superclass, log.close
begin; log.level = :nope; rescue ArgumentError => e; puts e.message; end
begin; Logger.new("app.log"); rescue ArgumentError => e; puts e.message; end
p Logger::Formatter.new.call("INFO", Time.utc(2020, 1, 2, 3, 4, 5, 123456), "p", "m"), Logger::Formatter.new.call("WARN", Time.utc(2020, 1, 2), nil, nil)
f = Logger::Formatter.new
f.datetime_format = "%d"
p f.call("ERROR", Time.utc(2020, 1, 2), "x", RuntimeError.new("e"))
`)
	out = strings.ReplaceAll(out, "#"+strconv.Itoa(os.Getpid())+"]", "#PID]")
	// The hour of l2's line is the test's own.
	out = strings.ReplaceAll(out, "I, ["+time00()+" #PID]  INFO -- svc: yes", "I, [HH #PID]  INFO -- svc: yes")
	assert.Equals(t, out, `0
true
true
nil
nil
"%Y"
"1.6.4"
0
5
["DEBUG", "INFO", "WARN", "ERROR", "FATAL", "ANY"]
I, [`+year()+` #PID]  INFO -- : hello
W, [`+year()+` #PID]  WARN -- : from block
E, [`+year()+` #PID] ERROR -- prog: added
E, [`+year()+` #PID] ERROR -- : boom (RuntimeError)

D, [`+year()+` #PID] DEBUG -- : [1, 2]
A, [`+year()+` #PID]   ANY -- : u
I, [`+year()+` #PID]  INFO -- : nil
I, [`+year()+` #PID]  INFO -- : :sym
W, [`+year()+` #PID]  WARN -- : blk
I, [`+year()+` #PID]  INFO -- p: m
A, [`+year()+` #PID]   ANY -- : nilsev
W, [`+year()+` #PID]  WARN -- pg: blockm
A, [`+year()+` #PID]   ANY -- : high
I, [`+year()+` #PID]  INFO -- : log
I, [`+year()+` #PID]  INFO -- app: with prog
2
false
true
true
W, [`+year()+` #PID]  WARN -- app: kept
3
raw
INFO|app|"custom"|Time
INFO|app2|"blockmsg"|Time
DEBUG|app|42|Time
0
true
I, [HH #PID]  INFO -- svc: yes
"svc"
1
1
RuntimeError
nil
invalid log level: nope
Logger.new("app.log"): cameo writes no files; log to STDOUT, STDERR or nil
"I, [2020-01-02T03:04:05.123456 #PID]  INFO -- p: m\n"
"W, [2020-01-02T00:00:00.000000 #PID]  WARN -- : nil\n"
"E, [02 #PID] ERROR -- x: e (RuntimeError)\n\n"
`)
}

func year() string   { return strconv.Itoa(timeNow().Year()) }
func time00() string { return timeNow().Format("15") }

func timeNow() time.Time { return time.Now() }
