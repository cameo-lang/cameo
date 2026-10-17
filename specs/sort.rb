#!/usr/bin/env ruby
# frozen_string_literal: true

# Sorts a checkout of ruby/spec into what amber's checker accepts, without
# running a spec:
#
#   supported/           amber check accepts the file
#   bug/                 amber's parser rejects Ruby that MRI accepts
#   unsupported/<what>/  the checker rejects a construct amber leaves out,
#                        or the file requires a library amber does not build in
#
# Each directory mirrors ruby/spec's tree, so a spec keeps its path, and
# carries the support files its specs load: spec_helper.rb, the top-level
# fixtures/ and shared/, the fixtures/ and shared/ next to each spec, and
# whatever a spec loads with require_relative, followed through. A file is
# filed under the first thing that stops it; the require_relative lines that
# load the harness, spec_helper and fixtures, are the runner's business and
# are not counted. MANIFEST.tsv lists every spec, where it went and why.
#
#   ruby sort.rb --spec PATH_TO_RUBY_SPEC [--amber BIN] [--out DIR]

require "fileutils"
require "open3"
require "optparse"
require "tmpdir"

HERE = __dir__
options = { out: HERE }
OptionParser.new do |o|
  o.banner = "usage: ruby sort.rb --spec DIR [--amber BIN] [--out DIR]"
  o.on("--spec DIR", "a checkout of github.com/ruby/spec") { options[:spec] = File.expand_path(_1) }
  o.on("--amber BIN", "the amber binary (default: build it)") { options[:amber] = File.expand_path(_1) }
  o.on("--out DIR", "where the sorted trees go (default: next to this script)") { options[:out] = File.expand_path(_1) }
end.parse!
abort "sort.rb: --spec is required" unless options[:spec]
SPEC = options[:spec]
OUT = options[:out]

amber = options[:amber] || begin
  bin = File.join(Dir.tmpdir, "amber-sort")
  system("go", "build", "-o", bin, "./src/cmd/amber", chdir: File.expand_path("..", HERE)) or abort "sort.rb: building amber failed"
  bin
end

# The libraries amber's require loads; any other is a LoadError.
BUILTIN = %w[set time pp pathname cgi cgi/escape cgi/util].freeze

# reason maps one of the checker's messages to the directory it files a spec
# under, or nil for the harness's require_relative.
def reason(msg)
  case msg
  when /\Arequire_relative is not supported/ then nil
  when /\Aparse error/ then "parse-error"
  when /\A@\w+( = …)?: amber has no instance variables|\Ainstance_variable_[gs]et is not supported/
    "instance-variables"
  when /\Aglobal variables are not supported/ then "globals"
  when /\Aclass variables are not supported/ then "class-variables"
  when /\Amultiple assignment binds local variables only/ then "multiple-assignment"
  when /changes its receiver|a bang method changes its receiver|the left side of an assignment must start with a local variable|\Aindex assignment takes one index|cannot store into its hash|\Aself\.\w+ = …: a field is set in initialize only|\Aself\[…\] = …: amber has no setters/
    "mutation"
  when /\Aclass << self is not supported|\Adef \S+ on an object|\Adef self\.\S+ outside a class body/ then "singleton-methods"
  when /\A(send|public_send|__send__|eval|class_eval|module_eval|instance_eval|instance_exec|class_exec|define_method|undef|method|binding) is not supported/
    "metaprogramming"
  when /\Aattr(_reader|_writer|_accessor)? is not supported|setters are not supported/ then "attr-accessor"
  when /\A(public|private|protected|module_function) is not supported/ then "visibility"
  when /\A(load|require_relative|autoload) is not supported/ then "require"
  when /\Abacktick commands are not supported|\A(system|exec|spawn) is not supported/ then "shell"
  when /\A(fork) is not supported/ then "processes"
  when /\A(rational|imaginary) literals are not supported/ then "complex"
  when /\Aassignment to a scoped constant/ then "scoped-constants"
  when /\A(lambda|proc) takes a block/ then "procs"
  when /\Aregexp|This regexp is not supported in Amber/ then "regexp"
  else "other"
  end
end

def requires(file)
  File.read(file, encoding: "UTF-8").scan(/^\s*require\s*\(?\s*['"]([^'"]+)['"]/).flatten
rescue ArgumentError
  []
end

def classify(amber, file)
  out, = Open3.capture2e(amber, "check", file, chdir: SPEC)
  messages = out.force_encoding("UTF-8").scrub.lines.filter_map { _1[/\A[^:]+:\d+: (.*)$/, 1] }
  lib = requires(File.join(SPEC, file)).find { !BUILTIN.include?(_1) }
  messages.each do |msg|
    why = reason(msg)
    next unless why
    # A require of a library amber lacks stops the file before anything
    # the checker found further down.
    return ["unsupported/require", "require '#{lib}'"] if lib && why != "parse-error"
    if why == "parse-error"
      _, status = Open3.capture2e("ruby", "-c", file, chdir: SPEC)
      return status.success? ? ["bug", msg] : ["unsupported/newer-syntax", msg]
    end
    return ["unsupported/#{why}", msg]
  end
  return ["unsupported/require", "require '#{lib}'"] if lib
  ["supported", ""]
end

# support lists the files a spec needs besides itself, as paths in SPEC.
def support(file, seen = {})
  dir = File.dirname(file)
  %w[fixtures shared].each do |sub|
    Dir.glob(File.join(SPEC, dir, sub, "**", "*"), File::FNM_DOTMATCH).each do |f|
      seen[f.delete_prefix("#{SPEC}/")] = true if File.file?(f)
    end
  end
  src = File.read(File.join(SPEC, file), encoding: "UTF-8") rescue ""
  src.scan(/require_relative\s*\(?\s*['"]([^'"]+)['"]/).flatten.each do |rel|
    target = File.expand_path(File.join(dir, rel), "/")[1..]
    target += ".rb" unless target.end_with?(".rb")
    next if seen[target] || !File.file?(File.join(SPEC, target))
    seen[target] = true
    support(target, seen)
  end
  seen
end

ROOT_SUPPORT = (%w[spec_helper.rb default.mspec .mspec.constants LICENSE] +
  Dir.glob("{fixtures,shared}/**/*", File::FNM_DOTMATCH, base: SPEC).select { File.file?(File.join(SPEC, _1)) }).freeze

specs = Dir.glob("**/*_spec.rb", base: SPEC).sort
rows = specs.map { |f| [*classify(amber, f), f] }

trees = rows.group_by(&:first)
%w[supported bug unsupported].each { FileUtils.rm_rf(File.join(OUT, _1)) }
trees.each do |category, entries|
  base = File.join(OUT, category)
  files = {}
  ROOT_SUPPORT.each { files[_1] = true }
  entries.each do |(_, _, f)|
    files[f] = true
    files.merge!(support(f))
  end
  files.each_key do |f|
    dest = File.join(base, f)
    FileUtils.mkdir_p(File.dirname(dest))
    FileUtils.cp(File.join(SPEC, f), dest)
  end
end

commit = `git -C #{SPEC} rev-parse HEAD`.strip
File.write(File.join(OUT, "MANIFEST.tsv"),
           (["directory\tspec\treason"] + rows.map { |c, why, f| [c, f, why.tr("\t", " ")].join("\t") }).join("\n") + "\n")
counts = trees.transform_values(&:size).sort_by { |k, v| [k == "supported" ? 0 : k == "bug" ? 1 : 2, -v] }
File.write(File.join(OUT, "COUNTS.md"), <<~MD)
  # ruby/spec sorted by amber's checker

  ruby/spec at commit `#{commit}`, #{rows.size} spec files, sorted by `sort.rb`
  without running a spec. See [README.md](README.md).

  | Directory | Specs |
  |---|---|
  #{counts.map { |k, v| "| `#{k}/` | #{v} |" }.join("\n")}
MD
warn counts.map { |k, v| "#{v}\t#{k}" }.join("\n")
