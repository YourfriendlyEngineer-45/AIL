package evaluator

import (
	"ail/internal/ast"
	"ail/internal/providers"
	"encoding/json"
	"testing"
)

func parse(t *testing.T, s string) ast.Program {
	t.Helper()
	p, e := ast.Parse([]byte(s))
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestCore(t *testing.T) {
	p := parse(t, `{"version":1,"program":[{"let":{"name":"x","value":{"literal":2}}},{"assert":{"left":{"binary":{"op":"+","left":{"var":"x"},"right":{"literal":3}}},"right":{"literal":5}}}]}`)
	if _, e := New(providers.MockProvider{}).Run(p); e != nil {
		t.Fatal(e)
	}
}
func TestStructured(t *testing.T) {
	v, e := Structured(`{"name":"x"}`, map[string]interface{}{"type": "object", "required": []interface{}{"name"}, "properties": map[string]interface{}{"name": map[string]interface{}{"type": "string"}}})
	if e != nil {
		t.Fatal(e)
	}
	if v.(map[string]interface{})["name"] != "x" {
		t.Fatal("bad output")
	}
}
func TestSchemaRejectsUnknown(t *testing.T) {
	_, e := ast.Parse([]byte(`{"version":1,"program":[{"let":{"name":"x","value":{"literal":1},"wat":1}}]}`))
	if e == nil {
		t.Fatal("expected schema error")
	}
}
func TestMemoryContextStream(t *testing.T) {
	m := NewMemory("")
	m.Data["x"] = "y"
	c := &ContextWindow{Limit: 5}
	c.Add("abc")
	c.Add("def")
	if c.Text() != "def" {
		t.Fatal("context limit")
	}
	_ = json.Valid
}
