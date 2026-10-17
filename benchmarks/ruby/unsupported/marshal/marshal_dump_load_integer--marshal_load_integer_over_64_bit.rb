smallint_array = 1000.times.map { |x| x }
bigint32_array = 1000.times.map { |x| x + 2**32 }
bigint64_array = 1000.times.map { |x| x + 2**64 }

smallint_dump = Marshal.dump(smallint_array)
bigint32_dump = Marshal.dump(bigint32_array)
bigint64_dump = Marshal.dump(bigint64_array)

Marshal.load(bigint64_dump)

