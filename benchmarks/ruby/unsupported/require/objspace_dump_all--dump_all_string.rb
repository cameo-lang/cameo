require 'objspace'
require 'tempfile'
$objs = 1_000.times.map { Object.new }
$strings = 1_000.times.map { |i| "string #{i}" }
$file = Tempfile.new('heap')
$dev_null = File.open(File::NULL, 'w+')

ObjectSpace.dump_all(output: :string)
