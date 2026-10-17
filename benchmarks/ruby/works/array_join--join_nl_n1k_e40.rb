# All elements are 7-bit ASCII Strings (the dominant real-world join case).
# Distinct objects so large-N cases exercise realistic heap/cache behavior.
a100_1    = Array.new(100)    { "a" * 1  }
a100_8    = Array.new(100)    { "a" * 8  }
a100_40   = Array.new(100)    { "a" * 40 }
a1k_1     = Array.new(1000)   { "a" * 1  }
a1k_8     = Array.new(1000)   { "a" * 8  }
a1k_40    = Array.new(1000)   { "a" * 40 }
a100k_1   = Array.new(100000) { "a" * 1  }
a100k_8   = Array.new(100000) { "a" * 8  }
a100k_40  = Array.new(100000) { "a" * 40 }

#--- benchmark: the prelude above runs once, the script below in the loop ---
a1k_40.join("\n")
