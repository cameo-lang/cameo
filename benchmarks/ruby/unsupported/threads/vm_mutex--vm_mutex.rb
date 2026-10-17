require 'thread'

m = Thread::Mutex.new

m.synchronize{}

