# frozen_string_literal: true
time = Time.now

#--- benchmark: the prelude above runs once, the script below in the loop ---
time.strftime("%FT%T.%3N")
