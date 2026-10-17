# frozen_string_literal: true
require 'cgi/escape'

long_all_escape = "'&\"<>" * 10
#--- benchmark: the prelude above runs once, the script below in the loop ---
CGI.escapeHTML(long_all_escape)
