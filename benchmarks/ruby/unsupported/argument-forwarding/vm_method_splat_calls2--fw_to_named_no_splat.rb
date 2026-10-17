def named_arg_splat(*a) end
def named_arg_kw_splat(*a, **kw) end
def anon_arg_splat(*) end
def anon_kw_splat(**) end
def anon_arg_kw_splat(*, **) end
def anon_fw_to_named(*, **) named_arg_kw_splat(*, **) end
def fw_to_named(...) named_arg_kw_splat(...) end
def fw_to_anon_to_named(...) anon_fw_to_named(...) end
def fw_no_kw(...) named_arg_splat(...) end
a = [1]
kw = {y: 1}

fw_to_named(1, y: 1)
