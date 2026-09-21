package evaluator

import (
	"ail/internal/ast"
	"encoding/json"
)

// Engineer is a bounded capability layer for an external AI engineer. It only accepts Ail JSON.
type Engineer struct {
	Engine     *Engine
	MaxRepairs int
	Repairs    int
}

func NewEngineer(e *Engine, maxRepairs int) *Engineer {
	if maxRepairs < 1 {
		maxRepairs = 3
	}
	return &Engineer{Engine: e, MaxRepairs: maxRepairs}
}
func (x *Engineer) Read(data []byte) (ast.Program, error) { return ast.Parse(data) }
func (x *Engineer) Validate(data []byte) error            { _, e := ast.Parse(data); return e }
func (x *Engineer) Run(data []byte) (interface{}, error) {
	p, e := x.Read(data)
	if e != nil {
		return nil, e
	}
	return x.Engine.Run(p)
}
func (x *Engineer) ApplyPatch(data []byte, patch func(ast.Program) ast.Program) ([]byte, error) {
	if x.Repairs >= x.MaxRepairs {
		return nil, Error{"AI_ERROR", "repair limit exceeded", ""}
	}
	p, e := x.Read(data)
	if e != nil {
		return nil, e
	}
	p = patch(p)
	out, e := ast.Canonical(p)
	if e != nil {
		return nil, e
	}
	if e = x.Validate(out); e != nil {
		return nil, e
	}
	x.Repairs++
	return out, nil
}
func (x *Engineer) Diagnose(err error) map[string]string {
	if err == nil {
		return map[string]string{"status": "pass"}
	}
	return map[string]string{"status": "fail", "error": err.Error()}
}
func (x *Engineer) Test(data []byte) (bool, error) {
	p, e := x.Read(data)
	if e != nil {
		return false, e
	}
	_, e = x.Engine.Run(p)
	return e == nil, e
}
func JSONPatchReplaceVersion(data []byte) []byte {
	var v map[string]interface{}
	if json.Unmarshal(data, &v) != nil {
		return data
	}
	v["version"] = 1
	b, _ := json.Marshal(v)
	return b
}
