# frozen_string_literal: true
require 'erb'

long_no_escape = "abcde" * 300
ERB::Util.html_escape(long_no_escape)
