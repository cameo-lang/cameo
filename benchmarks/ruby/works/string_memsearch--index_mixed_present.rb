# Haystacks of various sizes
small_hay  = "a" * 256
medium_hay = "a" * 4096
large_hay  = "a" * 65536

# Short needles (2-8 bytes) that exercise rb_memsearch_ss
needle_2 = "xy"
needle_4 = "xyzw"
needle_8 = "xyzwabcd"

# Needle whose first byte is absent from the haystack (memchr fast-path)
# vs needle whose first byte is common (rolling hash comparison)
first_byte_absent  = "x" + "a" * 3
first_byte_common  = "a" + "x" * 3

# Haystack with match at the end
hay_match_end = "a" * 4095 + "xy"

# Haystack with match at the start
hay_match_start = "xy" + "a" * 4094

# Mixed content haystack (more realistic)
mixed_hay = (("abcdefghij" * 100) + "z") * 10

mixed_hay.index("ijab")

