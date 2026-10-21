#!/usr/bin/env ruby
# frozen_string_literal: true

# Clones github.com/ruby/spec into ruby-spec/ at the root of the main
# repository, unless a copy is there already, and prints its path. Every
# worktree of the repository shares that one copy, so the default directory
# comes from git's common directory, not from the worktree the script runs in.
# An existing copy is left as it is: the script neither fetches nor checks out.
#
#   ruby specs/checkout.rb [--dir DIR] [--commit SHA]
#
# The path it prints is what sort.rb takes as --spec:
#
#   ruby specs/sort.rb --spec "$(ruby specs/checkout.rb)"

require "open3"
require "optparse"

URL = "https://github.com/ruby/spec"

# main_repository returns the root of the repository whose worktree holds
# this script: the parent of git's common directory, which is the same for
# every worktree.
def main_repository
  out, err, status = Open3.capture3("git", "rev-parse", "--path-format=absolute", "--git-common-dir", chdir: __dir__)
  abort "checkout.rb: git rev-parse failed: #{err}" unless status.success?
  File.dirname(out.strip)
end

options = {}
OptionParser.new do |o|
  o.banner = "usage: ruby checkout.rb [--dir DIR] [--commit SHA]"
  o.on("--dir DIR", "where the copy lives (default: ruby-spec/ in the main repository)") { options[:dir] = File.expand_path(_1) }
  o.on("--commit SHA", "the commit to check out after cloning (default: the default branch)") { options[:commit] = _1 }
end.parse!

dir = options[:dir] || File.join(main_repository, "ruby-spec")

if File.exist?(dir)
  abort "checkout.rb: #{dir} exists but is not a git repository" unless File.exist?(File.join(dir, ".git"))
else
  system("git", "clone", "--quiet", URL, dir, out: :err) or abort "checkout.rb: git clone #{URL} failed"
  if options[:commit]
    system("git", "-C", dir, "checkout", "--quiet", options[:commit], out: :err) or abort "checkout.rb: git checkout #{options[:commit]} failed"
  end
end

puts dir
