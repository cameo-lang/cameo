# frozen_string_literal: true
Warning[:experimental] = false
string = "The quick brown fox jumped over the lazy dog."
buffer = IO::Buffer.for(string)
format = [:U32, :U32, :U32, :U32]

[
  buffer.get_value(:U32, 0),
  buffer.get_value(:U32, 4),
  buffer.get_value(:U32, 8),
  buffer.get_value(:U32, 12),
]

