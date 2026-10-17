# frozen_string_literal: true
Warning[:experimental] = false
string = "The quick brown fox jumped over the lazy dog."
array = string.bytes
buffer = IO::Buffer.for(string)

upcased = String.new
string.each_byte do |byte|
  upcased << (byte ^ 32)
end

