class Range
  unless method_defined?(:overlap?)
    def overlap?(other)
      other.begin == self.begin || cover?(other.begin) || other.cover?(self.begin)
    end
  end
end

(2..3).overlap?(2..1)
