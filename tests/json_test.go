package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// runScript runs src and returns what it printed, failing the test when
// the script fails. Where a script is plain Ruby, the expected output is
// Ruby 3.4's.
func runScript(t *testing.T, src string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if err := cameo.Run("t.cameo", src, &stdout, &stderr); err != nil {
		t.Fatalf("%s\nfailed: %v\nstderr: %s", src, err, stderr.String())
	}
	return stdout.String()
}

func TestJSONParsesBigIntegersAsIntegers(t *testing.T) {
	out := runScript(t, `p JSON.parse("[123456789012345678901234567890, -9223372036854775809]").map(&:class)`)
	assert.Equals(t, out, "[Integer, Integer]\n")
}

func TestJSONParseSymbolizesNamesAtEveryDepth(t *testing.T) {
	out := runScript(t, `p JSON.parse('{"a": [{"b": 1}]}', symbolize_names: true)`)
	assert.Equals(t, out, "{a: [{b: 1}]}\n")
}

func TestJSONParseKeepsTheLastOfDuplicateKeys(t *testing.T) {
	assert.Equals(t, runScript(t, `p JSON.parse('{"a":1,"b":2,"a":3}')`), "{\"a\" => 3, \"b\" => 2}\n")
}

func TestJSONParserErrorNamesTheValueThatBroke(t *testing.T) {
	out := runScript(t, `
['{"a": {"b" 1}}', '[{"b" 1}]', '{"a": [1 2]}', "[1, /x]", "1 // no newline"].each do |src|
  JSON.parse(src)
rescue JSON::ParserError => e
  puts e.message
end`)
	assert.Equals(t, out, strings.Join([]string{
		`unexpected token at '{"a": {"b" 1}}'`,
		`unexpected token at '{"b" 1}]'`,
		`unexpected token at '2]}'`,
		`unexpected token at 'x]'`,
		`unexpected token at ''`,
	}, "\n")+"\n")
}

func TestJSONParseDecodesAHighSurrogateWithAnyFollowingEscape(t *testing.T) {
	assert.Equals(t, runScript(t, `p JSON.parse('"\\ud83d\\u0041"'), JSON.parse('"\\ude00"').bytes`), "\"👁\"\n[237, 184, 128]\n")
}

func TestJSONNestingLimitsDifferInParseAndGenerate(t *testing.T) {
	out := runScript(t, `
begin
  JSON.parse("[" * 101 + "]" * 101)
rescue JSON::NestingError => e
  puts e.message
end
begin
  JSON.generate([[[[1]]]], max_nesting: 3)
rescue JSON::NestingError => e
  puts e.message
end
p JSON.parse("[" * 101 + "]" * 101, max_nesting: false).flatten`)
	assert.Equals(t, out, "nesting of 101 is too deep\nnesting of 3 is too deep\n[]\n")
}

func TestJSONIndentsTheCloseOnlyAfterANewline(t *testing.T) {
	assert.Equals(t, runScript(t, `puts [1, {a: 2}].to_json(indent: "  ")`), "[  1,  {    \"a\":2}]\n")
}

func TestJSONPassesTheStateToAClassToJSON(t *testing.T) {
	out := runScript(t, `
class Pair
  int :a, :b
  def to_json(*args) = [a, b].to_json(*args)
end
puts JSON.pretty_generate({pair: Pair.new(a: 1, b: 2)})`)
	assert.Equals(t, out, "{\n  \"pair\": [\n    1,\n    2\n  ]\n}\n")
}

func TestJSONWritesAnInstanceAsItsFields(t *testing.T) {
	out := runScript(t, `
class Point
  int :x, :y
end
puts JSON.generate([Point.new(x: 1)])`)
	assert.Equals(t, out, "[{\"x\":1,\"y\":null}]\n")
}

func TestJSONDumpTakesALimit(t *testing.T) {
	out := runScript(t, `
begin
  JSON.dump([[[1]]], 2)
rescue ArgumentError => e
  puts e.message
end`)
	assert.Equals(t, out, "exceed depth limit\n")
}
