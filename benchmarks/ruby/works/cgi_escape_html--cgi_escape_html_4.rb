# frozen_string_literal: true
require 'cgi/escape'

long_no_escape = "abcde" * 300
CGI.escapeHTML(long_no_escape)
