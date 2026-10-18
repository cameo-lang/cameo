#!/usr/bin/env ruby
# frozen_string_literal: true

# Runs compare.rb on the cases where cameo is furthest behind MRI, to time
# work on them without the whole suite:
#
#   small hash literals built in a loop    hash_small8, hash_literal_small8, hash_small4
#   sorting that dispatches <=> per pair   array_sort_float, array_sort_block,
#                                          array_sort_string
#   method calls in deep recursion         app_tak, app_tarai, app_fib
#   blocks called in a loop                loop_each
#
# A case is CRuby's, from ../ruby/works, or cameo's own, from cases/.
#
#   ruby hotspots.rb [compare.rb's options]
#
# The rows go to hotspots.tsv unless --out says otherwise.

HOTSPOTS = %w[
  hash_small8 hash_literal_small8 hash_small4
  array_sort_float array_sort_block array_sort_string
  app_tak app_tarai app_fib
  loop_each--loop_each
].freeze

dirs = [File.expand_path("../ruby/works", __dir__), File.join(__dir__, "cases")]
files = HOTSPOTS.map { |c| dirs.map { File.join(_1, "#{c}.rb") }.find { File.exist?(_1) } || c }
missing = files.reject { File.exist?(_1) }
abort "hotspots.rb: no such case: #{missing.join(", ")}" unless missing.empty?
out = ARGV.include?("--out") ? [] : ["--out", File.join(__dir__, "hotspots.tsv")]
exec("ruby", File.join(__dir__, "compare.rb"), *out, *ARGV, *files)
