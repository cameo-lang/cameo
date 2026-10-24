#!/usr/bin/env ruby
# frozen_string_literal: true

# Clones github.com/ruby/spec into ruby-spec/ at the root of the main
# repository, or pulls into the copy that is there already, and prints its
# path. Every worktree of the repository shares that one copy, so the default
# directory comes from git's common directory, not from the worktree the
# script runs in. With --commit, the copy is fetched and that commit checked
# out in place of a pull.
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
  o.on("--commit SHA", "the commit to check out in place of a pull (default: pull the current branch)") { options[:commit] = _1 }
end.parse!

dir = options[:dir] || File.join(main_repository, "ruby-spec")

# git runs a git command in dir, with its output on stderr so that stdout
# holds the path alone, and aborts when it fails.
def git(dir, *args)
  system("git", "-C", dir, *args, out: :err) or abort "checkout.rb: git #{args.join(" ")} failed in #{dir}"
end

if !File.exist?(dir)
  system("git", "clone", "--quiet", URL, dir, out: :err) or abort "checkout.rb: git clone #{URL} failed"
elsif !File.exist?(File.join(dir, ".git"))
  abort "checkout.rb: #{dir} exists but is not a git repository"
elsif options[:commit]
  git(dir, "fetch", "--quiet")
else
  # --ff-only: a copy with local commits is left for its owner to reconcile.
  git(dir, "pull", "--quiet", "--ff-only")
end
git(dir, "checkout", "--quiet", options[:commit]) if options[:commit]

puts dir
