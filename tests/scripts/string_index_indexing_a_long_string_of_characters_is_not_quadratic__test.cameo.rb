# string index: testcase to demonstrate that indexing a long string of
# characters is not quadratic
#
# Reading a long string with characters beyond ASCII by index reads each
# character once: this loop made a walk from the start for each index, and
# took minutes at this length.
#
# (Introduced in PR #436. expected output follows the __END__ separator.)
s = "é" + "ab " * 200_000
i = 0
i += 1 until s[i].nil?
p i, s[i - 1], s[0]

__END__
600001
" "
"é"
