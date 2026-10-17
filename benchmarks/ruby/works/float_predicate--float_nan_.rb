floats = [1.0, -1.0, 0.0, Float::NAN, Float::INFINITY, -Float::INFINITY]

floats.each { |f| f.nan? }
