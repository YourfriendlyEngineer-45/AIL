package internal

import (
	"ail/internal/ast"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CompileGo emits a deterministic standalone Go representation of the canonical program.
func CompileGo(p ast.Program) ([]byte, error) {
	if _, e := ast.Canonical(p); e != nil {
		return nil, e
	}
	var b bytes.Buffer
	b.WriteString("package main\n\nimport \"fmt\"\n\nfunc main() {\n")
	emitStmts(&b, p.Program, "\t")
	b.WriteString("}\n")
	return b.Bytes(), nil
}
func emitStmts(b *bytes.Buffer, ss []ast.Stmt, ind string) {
	for _, s := range ss {
		for k, v := range s {
			switch k {
			case "expr":
				b.WriteString(ind + "fmt.Println(")
				b.WriteString(goExpr(v))
				b.WriteString(")\n")
			case "let":
				var x struct {
					Name  string          `json:"name"`
					Value json.RawMessage `json:"value"`
				}
				json.Unmarshal(v, &x)
				b.WriteString(ind + "// Ail let " + x.Name + " = " + string(x.Value) + "\n")
			case "return":
				b.WriteString(ind + "return\n")
			case "if":
				var x struct {
					Condition json.RawMessage `json:"condition"`
					Then      []ast.Stmt      `json:"then"`
				}
				json.Unmarshal(v, &x)
				b.WriteString(ind + "if " + goExpr(x.Condition) + " {\n")
				emitStmts(b, x.Then, ind+"\t")
				b.WriteString(ind + "}\n")
			}
		}
	}
}
func goExpr(v json.RawMessage) string { var x interface{}; json.Unmarshal(v, &x); return goVal(x) }
func goVal(x interface{}) string {
	switch v := x.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return "nil"
	case map[string]interface{}:
		if z, ok := v["literal"]; ok {
			return goVal(z)
		}
		if z, ok := v["var"]; ok {
			return fmt.Sprint(z)
		}
		if z, ok := v["binary"].(map[string]interface{}); ok {
			return "(" + goVal(z["left"]) + " " + fmt.Sprint(z["op"]) + " " + goVal(z["right"]) + ")"
		}
		if z, ok := v["call"].(map[string]interface{}); ok {
			return fmt.Sprint(z["name"]) + "()"
		}
	}
	return "nil"
}
func CompileJava(p ast.Program) ([]byte, error) {
	if _, e := ast.Canonical(p); e != nil {
		return nil, e
	}
	var b bytes.Buffer
	b.WriteString("public final class Main {\n  public static void main(String[] args) {\n")
	for _, s := range p.Program {
		for k, v := range s {
			if k == "expr" {
				b.WriteString("    System.out.println(")
				b.WriteString(javaExpr(v))
				b.WriteString(");\n")
			}
		}
	}
	b.WriteString("  }\n}\n")
	return b.Bytes(), nil
}
func javaExpr(v json.RawMessage) string { var x interface{}; json.Unmarshal(v, &x); return javaVal(x) }
func javaVal(x interface{}) string {
	switch v := x.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(v, "\"", "\\\""))
	case float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	case map[string]interface{}:
		if z, ok := v["literal"]; ok {
			return javaVal(z)
		}
	}
	return "null"
}
func SortedKeys(m map[string]interface{}) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
