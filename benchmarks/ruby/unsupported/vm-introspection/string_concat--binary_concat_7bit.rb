# frozen_string_literal: true
CHUNK = "a" * 64
UCHUNK = "é" * 32
SHORT = "a" * (GC.stat_heap(0, :slot_size) / 2)
LONG = "a" * (GC.stat_heap(0, :slot_size) * 2)
GC.disable # GC causes a lot of variance

buffer = String.new(capacity: 4096, encoding: Encoding::BINARY)
buffer << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK
buffer << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK
buffer << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK
buffer << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK
buffer << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK
buffer << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK
buffer << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK
buffer << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK << CHUNK

