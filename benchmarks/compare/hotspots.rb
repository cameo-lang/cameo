#!/usr/bin/env ruby
# frozen_string_literal: true

# Runs compare.rb on the cases where amber is furthest behind MRI, to time
# work on them without the whole suite:
#
#   small hash literals built in a loop    hash_small8, hash_literal_small8, hash_small4
#   sorting that dispatches <=> per pair   array_sort_float, array_sort_block
#   method calls in deep recursion         app_tak, app_tarai, app_fib
#   blocks called in a loop                loop_each
#
#   ruby hotspots.rb [compare.rb's options]
#
# The rows go to hotspots.tsv unless --out says otherwise.

HOTSPOTS = %w[
  hash_small8 hash_literal_small8 hash_small4
  array_sort_float array_sort_block
  app_tak app_tarai app_fib
  loop_each--loop_each
].freeze

works = File.expand_path("../ruby/works", __dir__)
files = HOTSPOTS.map { File.join(works, "#{_1}.rb") }
missing = files.reject { File.exist?(_1) }
abort "hotspots.rb: no such case: #{missing.join(", ")}" unless missing.empty?
out = ARGV.include?("--out") ? [] : ["--out", File.join(__dir__, "hotspots.tsv")]
exec("ruby", File.join(__dir__, "compare.rb"), *out, *ARGV, *files)
