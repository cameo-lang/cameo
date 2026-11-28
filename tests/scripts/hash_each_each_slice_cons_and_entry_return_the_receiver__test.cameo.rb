# hash each: testcase to demonstrate that each slice cons and entry return the
# receiver
#
# each_slice, each_cons and each_entry return the receiver when they are given
# a block, as Ruby's do since 3.1.
#
# (Introduced in PR #390. expected output follows the __END__ separator.)
p [1, 2, 3].each_slice(2) { }, [1, 2, 3].each_cons(2) { }, [1, 2].each_entry { }
p (1..3).each_slice(2) { }, { a: 1 }.each_cons(1) { }, { a: 1 }.each_entry { |x| p x }

__END__
[1, 2, 3]
[1, 2, 3]
[1, 2]
[:a, 1]
1..3
{a: 1}
{a: 1}
