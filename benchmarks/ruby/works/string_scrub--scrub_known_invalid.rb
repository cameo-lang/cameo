
STRING_SIZE = 1024
def duplicate_to_length(str, target_length)
  return "" if target_length <= 0
  return str[0, target_length] if str.length >= target_length

  (str * ((target_length / str.length) + 1))[0, target_length]
end
base = "Hello \u{1f600} world! \u{00e9}\u{00f1}"
padding = duplicate_to_length(base, STRING_SIZE)

valid_utf8 = (padding.b + "OK".b).force_encoding("UTF-8")
valid_utf8.valid_encoding?
unknown_but_valid_utf8 = valid_utf8.dup.b.force_encoding("UTF-8")
invalid_utf8 = (padding.b + "\x80\xFF".b).force_encoding("UTF-8")
invalid_utf8.valid_encoding?
unknown_but_invalid_utf8 = (padding.b + "\x80\xFF".b).force_encoding("UTF-8")

worst_case_utf8 = duplicate_to_length("\u{1f600}\u{00e9}\u{00f1}", STRING_SIZE).b.force_encoding("UTF-8")

unknown_but_valid_utf8_worst_case = worst_case_utf8.dup.b.force_encoding("UTF-8")
unknown_but_invalid_utf8_worst_case = (worst_case_utf8.b + "\x80\xFF".b).force_encoding("UTF-8")

#--- benchmark: the prelude above runs once, the script below in the loop ---
string = invalid_utf8.dup
string.scrub!

