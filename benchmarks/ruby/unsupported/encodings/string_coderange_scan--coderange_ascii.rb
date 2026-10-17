def unknown(s) = s.b.force_encoding("UTF-8")
multibyte   = unknown("\u{00e9}" * 16384)   # best case: every byte non-ASCII
alternating = unknown("\u{00e9}a" * 10922)  # worst case: non-ASCII then ASCII
ascii       = unknown("a" * 32768)          # baseline

ascii.dup.valid_encoding?
