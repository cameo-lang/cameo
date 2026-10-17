# frozen_string_literal: true
CHUNK = "a" * 64
UCHUNK = "é" * 32
SHORT = "a" * (GC.stat_heap(0, :slot_size) / 2)
LONG = "a" * (GC.stat_heap(0, :slot_size) * 2)
GC.disable # GC causes a lot of variance

# Concat 20 times in a String with 23 capacity
# Excercise `str_embed_capa`
buffer = +""
buffer << "1" << "2" << "3" << "4" << "5" << "6" << "7" << "8" << "9" << "0"
buffer << "1" << "2" << "3" << "4" << "5" << "6" << "7" << "8" << "9" << "0"

