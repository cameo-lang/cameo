# attributes: testcase to demonstrate that a class body sets attributes and is
# sealed after
#
# Class attributes are cameo's own; these outputs are their specification. A
# class is built in its body as an instance is in initialize: self.name = v
# sets an attribute while the body runs, from the body or from a class method
# it calls, and the class is sealed when the body ends.
#
# (Introduced in PR #249. expected output follows the __END__ separator.)
class Switches
  attribute :switches, default: []
  attribute :label

  def self.on(name, &step)
    self.switches += [[name, step]]
  end
  def self.names = switches.map { |n, _| n }
end
class Options < Switches
  bool :verbose, default: false
  string :name, default: "world"
  on("--verbose") { |o, _| o.with(verbose: true) }
  on("--name") { |o, arg| o.with(name: arg.to_s) }
  self.label = "opts"
end
p Switches.names, Options.names, Switches.label, Options.label
o = Options.switches.reduce(Options.new) { |o, (_, step)| step.call(o, nil) }
p o
begin
  Options.on("--late") { }
rescue CameoError => e
  puts e.message
end
begin
  Options.new.class.on("--late") { }
rescue CameoError => e
  puts e.message
end

__END__
[]
["--verbose", "--name"]
nil
"opts"
Options(verbose: true, name: "")
self.switches = …: Options is sealed: an attribute is set while the class body runs
self.switches = …: Options is sealed: an attribute is set while the class body runs
