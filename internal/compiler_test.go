package internal

import (
	"ail/internal/ast"
	"strings"
	"testing"
)

func TestDeterministicTargets(t *testing.T) {
	p := ast.Program{Version: 1, Program: []ast.Stmt{{"expr": []byte(`{"call":{"name":"print","args":[{"literal":"x"}]}}`)}}}
	a, e := CompileGo(p)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := CompileGo(p)
	if string(a) != string(b) {
		t.Fatal("Go output is not deterministic")
	}
	j, e := CompileJava(p)
	if e != nil || !strings.Contains(string(j), "class Main") {
		t.Fatal("Java generation failed")
	}
}
