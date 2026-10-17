# frozen_string_literal: true
Warning[:experimental] = false
string = "The quick brown fox jumped over the lazy dog."
buffer = IO::Buffer.for(string)
format = [:U32, :U32, :U32, :U32]

buffer.get_values(format, 0)

