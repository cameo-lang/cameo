ary = []
kw = {a: 1}
empty_kw = {}
kw_ary = [Hash.ruby2_keywords_hash(a: 1)]
empty_kw_ary = [Hash.ruby2_keywords_hash({})]

itself(*ary, **empty_kw)
