# frozen_string_literal: true
Warning[:experimental] = false
string = "The quick brown fox jumped over the lazy dog."
buffer = IO::Buffer.for(string)
format = [:U32, :U32, :U32, :U32]

[
  string.unpack1("N"),
  string.unpack1("N", offset: 4),
  string.unpack1("N", offset: 8),
  string.unpack1("N", offset: 12),
]

