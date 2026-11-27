package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// attr :name, type: X declares a typed field whose type is a class or the
// name of one; these outputs are its specification.

func TestAttrWithAClassTypeIsARefField(t *testing.T) {
	out := runScript(t, `
class UserName
  string :user_name

  def initialize(user_name:)
    self.user_name = user_name
  end

  def normalize = UserName.new(user_name: user_name.strip)
end

class User
  attr :name, type: UserName
end

user = User.new(name: UserName.new(user_name: " hugo "))
normalized_user = user.with { |u| { name: u.name.normalize } }
p normalized_user.name.user_name, user.name.user_name
p normalized_user == User.new(name: UserName.new(user_name: "hugo"))
`)
	assert.Equals(t, out, "\"hugo\"\n\" hugo \"\ntrue\n")
}

func TestAttrTakesAClassNameAndCoreClasses(t *testing.T) {
	out := runScript(t, `
module Shop
  class Item
    string :sku
  end
end
class Line
  attr :item, type: "Shop::Item"
  attr :qty, type: Integer, default: 1
  attr :label, type: String
  attr :price, :tax, type: Float
end
l = Line.new(item: Shop::Item.new(sku: "x"), label: "a", price: 2)
p l.item.sku, l.qty, l.label, l.price
p l.with { |x| { qty: x.qty + 2 } }.qty
begin
  Line.new(qty: "no")
rescue TypeError => e
  puts e.message
end
begin
  Line.new(item: "no")
rescue TypeError => e
  puts e.message
end
`)
	assert.Equals(t, out, "\"x\"\n1\n\"a\"\n2.0\n3\nfield qty of Line takes an int, not an instance of String\nfield item of Line takes a Shop::Item, not an instance of String\n")
}

func TestAttrRefusesWhatIsNoType(t *testing.T) {
	for src, want := range map[string]string{
		"class A\n  attr :x\nend\n":                  "attr without type: is not supported",
		"class A\n  attr :x, type: 3\nend\n":         "attr: type: is a class or the name of a class",
		"class A\n  attr :x, type: \"nope\"\nend\n":  "attr: type: \"nope\" is not a class name",
		"class A\n  attr :x, type: A, of: 1\nend\n":  "attr: unknown option of:",
		"class A\n  attr \"x\", type: String\nend\n": "attr: a field is named by a symbol",
		"attr :x, type: String\n":                    "attr :x declares a field, which only a class body can do",
		"class A\n  attr :x, type: Nope\nend\n":      "Nope",
	} {
		var stdout, stderr bytes.Buffer
		err := cameo.RunFromString("t.cameo", src, cameoio.Streams(&stdout, &stderr))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), want)
	}
}
