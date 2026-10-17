# frozen_string_literal: true
require 'cgi/escape'


#--- benchmark: the prelude above runs once, the script below in the loop ---
CGI.escapeHTML("abcd<")
