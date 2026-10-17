MarshalLoadPartialObjectsObject = Class.new do
  def initialize(a, b, c, d)
    @a = a
    @b = b
    @c = c
    @d = d
  end
end

MarshalLoadPartialObjectsStruct = Struct.new(:a, :b, :c, :d)
MarshalLoadPartialObjectsData = Data.define(:a, :b, :c, :d)

integer_array_10 = 10.times.to_a
integer_array_1000 = 1000.times.to_a
unique_strings = 1000.times.map { |i| "string #{i}" }
shared_string = +"shared string"
shared_strings = Array.new(1000, shared_string)
integer_hash = 1000.times.to_h { |i| [i, i] }
symbol_string_hash = 1000.times.to_h { |i| ["key#{i}".to_sym, "value #{i}"] }
objects = 1000.times.map { |i| MarshalLoadPartialObjectsObject.new(i, i + 1, i + 2, i + 3) }
structs = 1000.times.map { |i| MarshalLoadPartialObjectsStruct.new(i, i + 1, i + 2, i + 3) }
data_objects = 1000.times.map { |i| MarshalLoadPartialObjectsData.new(i, i + 1, i + 2, i + 3) }

integer_array_10_dump = Marshal.dump(integer_array_10)
integer_array_1000_dump = Marshal.dump(integer_array_1000)
unique_strings_dump = Marshal.dump(unique_strings)
shared_strings_dump = Marshal.dump(shared_strings)
integer_hash_dump = Marshal.dump(integer_hash)
symbol_string_hash_dump = Marshal.dump(symbol_string_hash)
objects_dump = Marshal.dump(objects)
structs_dump = Marshal.dump(structs)
data_objects_dump = Marshal.dump(data_objects)

load_proc = ->(object) { object }

Marshal.load(integer_array_1000_dump, freeze: true)
