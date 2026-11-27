# json: testcase to demonstrate that JSON passes the state to a class to JSON
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
class Pair
  int :a, :b
  def to_json(*args) = [a, b].to_json(*args)
end
puts JSON.pretty_generate({pair: Pair.new(a: 1, b: 2)})

__END__
{
  "pair": [
    1,
    2
  ]
}
