# string index: testcase to demonstrate that workers index strings at once
#
# Workers index strings at once: the index is one for all of them.
#
# (Introduced in PR #436. expected output follows the __END__ separator.)
include Cameo
class Reader
  def count(k)
    s = "é#{k}" + "aé" * 2_000
    n = 0
    i = 0
    until s[i].nil?
      n += 1 if s[i] == "é"
      i += 1
    end
    n
  end
end
r = Reader.new
p await((1..8).map { |k| async r, :count, k })

__END__
[2001, 2001, 2001, 2001, 2001, 2001, 2001, 2001]
