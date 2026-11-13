#!/usr/bin/env ruby
# frozen_string_literal: true

# Prints a results file of compare.rb as a table of categories: for each
# group of cases, how many ran on every Ruby of the file, and each Ruby's
# time as a multiple of MRI's, the geometric mean over those cases. The
# table is Markdown, as the README and the site show it.
#
#   ruby categories.rb [FILE]      # default: yjit.tsv beside this script
#
# A case counts when every Ruby ran it cleanly. A category is the first
# word of the case's file name, grouped by CATEGORIES.

CATEGORIES = {
  "Method calls, blocks and the VM" => %w[vm keyword loop kernel object nil nilclass gc ractor pm],
  "Small programs" => %w[app],
  "Arrays and Enumerable" => %w[array enum],
  "Hashes" => %w[hash],
  "Sets" => %w[set],
  "Strings and regexps" => %w[string regexp match],
  "Numbers" => %w[float int integer num numeric],
  "Ranges" => %w[range],
  "Time" => %w[time],
  "Libraries (File paths, Pathname, CGI)" => %w[file pathname cgi],
}.freeze

LABELS = { "mri" => "CRuby", "yjit" => "CRuby + YJIT", "cameo" => "cameo", "warmruby" => "warmruby", "rbgo" => "rbgo" }.freeze

file = ARGV[0] || File.join(__dir__, "yjit.tsv")
lines = File.readlines(file, chomp: true)
keys = lines.shift.split("\t")
rows = lines.map { keys.zip(_1.split("\t", -1)).to_h }
names = keys.filter_map { _1[/\A(\w+)_result\z/, 1] }

# The word a case's file name starts with: set--add.rb is of set,
# string_upcase--upcase-256.rb of string.
def word(file) = file[/\A[a-z0-9]+/]

def geomean(xs) = Math.exp(xs.sum { Math.log(_1) } / xs.size)

ran = rows.select { |r| names.all? { r["#{_1}_result"] == "ok" } }
known = CATEGORIES.values.flatten

puts "| Category | Cases | #{names.map { LABELS.fetch(_1) }.join(" | ")} |"
puts "|---|---:|#{names.map { "---:" }.join("|")}|"
line = lambda do |label, cases|
  cells = names.map do |name|
    format("%.2f×", geomean(cases.map { _1["#{name}_secs"].to_f / _1["mri_secs"].to_f }))
  end
  puts "| #{label} | #{cases.size} | #{cells.join(" | ")} |"
end
CATEGORIES.each do |label, words|
  cases = ran.select { words.include?(word(_1["file"])) }
  line.(label, cases) unless cases.empty?
end
# A case of a word no category names.
rest = ran.reject { known.include?(word(_1["file"])) }
line.("Other", rest) unless rest.empty?
line.("**All**", ran)
warn "#{ran.size} of #{rows.size} cases ran on every Ruby"
