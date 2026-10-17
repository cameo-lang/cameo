# frozen_string_literal: true
require 'erb'

long_all_escape = "'&\"<>" * 10
ERB::Util.html_escape(long_all_escape)
