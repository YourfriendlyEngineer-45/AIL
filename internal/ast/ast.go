package ast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type Program struct {
	Version int    `json:"version"`
	Program []Stmt `json:"program"`
}
type Stmt map[string]json.RawMessage
type SchemaError struct{ Path, Message string }

func (e SchemaError) Error() string { return "SCHEMA_ERROR: " + e.Path + ": " + e.Message }

var stmtFields = map[string]map[string]bool{
	"let": {"name": true, "value": true}, "set": {"name": true, "value": true}, "if": {"condition": true, "then": true, "else": true}, "while": {"condition": true, "body": true, "max": true}, "for": {"name": true, "list": true, "body": true}, "return": {"value": true}, "try": {"body": true, "catch": true, "error": true}, "function": {"name": true, "params": true, "body": true}, "import": {}, "prompt": {"name": true, "template": true}, "tool": {"name": true, "description": true, "schema": true, "implementation": true}, "agent": {"name": true, "instructions": true, "tools": true, "memory": true, "model": true, "max_steps": true}, "memory": {"name": true, "file": true}, "context": {"name": true, "limit": true}, "stream": {}, "expr": {}, "assert": {"left": true, "right": true, "message": true}, "export": {}}
var exprFields = map[string]map[string]bool{"literal": {}, "var": {}, "binary": {"op": true, "left": true, "right": true}, "unary": {"op": true, "value": true}, "call": {"name": true, "args": true, "output": true}, "list": {}, "map": {}, "index": {"target": true, "index": true}, "property": {"target": true, "name": true}}

func Parse(data []byte) (Program, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var raw map[string]json.RawMessage
	if e := dec.Decode(&raw); e != nil {
		return Program{}, SchemaError{"$", "invalid JSON: " + e.Error()}
	}
	if dec.More() {
		return Program{}, SchemaError{"$", "trailing JSON"}
	}
	if len(raw) != 2 {
		return Program{}, SchemaError{"$", "program must contain exactly version and program"}
	}
	var v int
	if e := json.Unmarshal(raw["version"], &v); e != nil || v != 1 {
		return Program{}, SchemaError{"version", "must be 1"}
	}
	var a []json.RawMessage
	if e := json.Unmarshal(raw["program"], &a); e != nil {
		return Program{}, SchemaError{"program", "must be an array"}
	}
	p := Program{1, nil}
	for i, b := range a {
		var s map[string]json.RawMessage
		if e := json.Unmarshal(b, &s); e != nil {
			return Program{}, SchemaError{fmt.Sprintf("program[%d]", i), "statement must be object"}
		}
		if e := ValidateStmt(s, fmt.Sprintf("program[%d]", i)); e != nil {
			return Program{}, e
		}
		p.Program = append(p.Program, s)
	}
	return p, nil
}
func ValidateStmt(s map[string]json.RawMessage, p string) error {
	if len(s) != 1 {
		return SchemaError{p, "statement must contain exactly one node"}
	}
	for k, v := range s {
		fields, ok := stmtFields[k]
		if !ok {
			return SchemaError{p, "unknown statement: " + k}
		}
		if len(v) == 0 {
			return SchemaError{p, k + " is empty"}
		}
		var x map[string]json.RawMessage
		if k == "import" || k == "return" || k == "export" || k == "expr" || k == "stream" {
			continue
		}
		if e := json.Unmarshal(v, &x); e != nil {
			return SchemaError{p + "." + k, "must be object"}
		}
		for f := range x {
			if !fields[f] {
				return SchemaError{p + "." + k, "unknown field: " + f}
			}
		}
		for req := range required(k) {
			if _, ok := x[req]; !ok {
				return SchemaError{p + "." + k, "missing required field: " + req}
			}
		}
		validateNested(k, x, p+"."+k)
	}
	return nil
}
func required(k string) map[string]bool {
	r := map[string]bool{}
	switch k {
	case "let", "set", "if", "while", "for", "function", "prompt", "tool", "agent", "memory", "context":
		for _, x := range map[string][]string{"let": {"name", "value"}, "set": {"name", "value"}, "if": {"condition", "then"}, "while": {"condition", "body"}, "for": {"name", "list", "body"}, "function": {"name", "params", "body"}, "prompt": {"name", "template"}, "tool": {"name", "description", "schema", "implementation"}, "agent": {"name", "instructions", "tools", "max_steps"}, "memory": {"name"}, "context": {"name"}}[k] {
			r[x] = true
		}
	}
	return r
}
func validateNested(k string, x map[string]json.RawMessage, p string) { _ = k; _ = x; _ = p }
func ValidateExpr(v json.RawMessage, p string) error {
	var x interface{}
	if e := json.Unmarshal(v, &x); e != nil {
		return SchemaError{p, "invalid JSON expression"}
	}
	switch x.(type) {
	case string, float64, bool, nil:
		return nil
	}
	m, ok := x.(map[string]interface{})
	if !ok {
		return SchemaError{p, "expression must be literal or node object"}
	}
	if len(m) != 1 {
		return SchemaError{p, "expression node must contain exactly one key"}
	}
	for k := range m {
		if _, ok := exprFields[k]; !ok {
			return SchemaError{p, "unknown expression: " + k}
		}
	}
	return nil
}
func Canonical(p Program) ([]byte, error) { return json.Marshal(p) }
func Keys(m map[string]json.RawMessage) []string {
	a := make([]string, 0, len(m))
	for k := range m {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}
