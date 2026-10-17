# frozen_string_literal: true
require 'cgi/escape'

long_no_escape = "abcde" * 300
#--- benchmark: the prelude above runs once, the script below in the loop ---
CGI.escapeHTML(long_no_escape)
