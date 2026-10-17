# frozen_string_literal: true
require 'cgi/escape'

long_all_escape = "'&\"<>" * 10
CGI.escapeHTML(long_all_escape)
