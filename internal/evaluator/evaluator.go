package evaluator

import (
	"ail/internal/ast"
	"ail/internal/providers"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type Error struct {
	Kind, Message string
	Path          string
}

func (e Error) Error() string {
	if e.Path != "" {
		return e.Kind + ": " + e.Path + ": " + e.Message
	}
	return e.Kind + ": " + e.Message
}

type Memory struct {
	Data map[string]interface{}
	File string
}

func NewMemory(file string) *Memory {
	m := &Memory{Data: map[string]interface{}{}, File: file}
	if file != "" {
		if b, e := os.ReadFile(file); e == nil {
			_ = json.Unmarshal(b, &m.Data)
		}
	}
	return m
}
func (m *Memory) Save() error {
	if m.File == "" {
		return nil
	}
	b, e := json.Marshal(m.Data)
	if e != nil {
		return e
	}
	return os.WriteFile(m.File, b, 0600)
}

type ContextWindow struct {
	Items []string
	Limit int
}

func (c *ContextWindow) Add(s string) { c.Items = append(c.Items, s); c.trim() }
func (c *ContextWindow) trim() {
	for len(strings.Join(c.Items, "\n")) > c.Limit && len(c.Items) > 0 {
		c.Items = c.Items[1:]
	}
}
func (c *ContextWindow) Text() string { return strings.Join(c.Items, "\n") }

type Tool struct {
	Name, Description string
	Schema            map[string]interface{}
	Impl              func(map[string]interface{}) (interface{}, error)
}
type Agent struct {
	Name, Instructions string
	Tools              map[string]Tool
	Memory             *Memory
	Provider           providers.Provider
	Model              string
	MaxSteps, Steps    int
}

type Engine struct {
	Vars      map[string]interface{}
	Functions map[string]map[string]interface{}
	Exports   map[string]bool
	Provider  providers.Provider
	Memory    *Memory
	Context   *ContextWindow
	Tools     map[string]Tool
	Agents    map[string]Agent
	Modules   map[string]ast.Program
	Imports   map[string]int
	Output    func(interface{})
	cwd       string
}

func New(p providers.Provider) *Engine {
	return &Engine{Vars: map[string]interface{}{}, Functions: map[string]map[string]interface{}{}, Exports: map[string]bool{}, Provider: p, Memory: NewMemory(""), Context: &ContextWindow{Limit: 4000}, Tools: map[string]Tool{}, Agents: map[string]Agent{}, Modules: map[string]ast.Program{}, Imports: map[string]int{}, Output: func(v interface{}) { fmt.Println(v) }}
}
func (e *Engine) Run(p ast.Program) (interface{}, error) { return e.runBlock(p.Program, e.Vars) }
func (e *Engine) SetCWD(dir string)                      { e.cwd = dir }
func (e *Engine) runBlock(stmts []ast.Stmt, env map[string]interface{}) (interface{}, error) {
	for _, s := range stmts {
		v, ret, err := e.exec(s, env)
		if err != nil {
			return nil, err
		}
		if ret {
			return v, nil
		}
	}
	return nil, nil
}
func raw(m map[string]json.RawMessage, k string) (json.RawMessage, bool) { v, ok := m[k]; return v, ok }
func strField(b json.RawMessage) (string, error) {
	var s string
	err := json.Unmarshal(b, &s)
	return s, err
}
func (e *Engine) exec(s ast.Stmt, env map[string]interface{}) (interface{}, bool, error) {
	for k, b := range s {
		switch k {
		case "let", "set":
			var x struct {
				Name  string          `json:"name"`
				Value json.RawMessage `json:"value"`
			}
			if err := json.Unmarshal(b, &x); err != nil || x.Name == "" {
				return nil, false, Error{"SCHEMA_ERROR", "invalid variable declaration", ""}
			}
			v, err := e.Expr(x.Value, env)
			if err != nil {
				return nil, false, err
			}
			if k == "let" {
				if _, ok := env[x.Name]; ok {
					return nil, false, Error{"SEMANTIC_ERROR", "variable already defined: " + x.Name, ""}
				}
			}
			env[x.Name] = v
		case "expr":
			v, err := e.Expr(b, env)
			if err != nil {
				return nil, false, err
			}
			e.Output(v)
		case "assert":
			var x struct {
				Left, Right json.RawMessage
				Message     string `json:"message"`
			}
			if err := json.Unmarshal(b, &x); err != nil {
				return nil, false, Error{"SCHEMA_ERROR", "invalid assert", ""}
			}
			a, err := e.Expr(x.Left, env)
			if err != nil {
				return nil, false, err
			}
			c, err := e.Expr(x.Right, env)
			if err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(a, c) {
				msg := x.Message
				if msg == "" {
					msg = "assertion failed"
				}
				return nil, false, Error{"TEST_FAILURE", msg, ""}
			}
		case "return":
			var x json.RawMessage
			if err := json.Unmarshal(b, &x); err != nil {
				return nil, false, Error{"SCHEMA_ERROR", "invalid return", ""}
			}
			v, err := e.Expr(x, env)
			return v, true, err
		case "if":
			var x struct {
				Cond       json.RawMessage `json:"condition"`
				Then, Else []ast.Stmt
			}
			if err := json.Unmarshal(b, &x); err != nil {
				return nil, false, Error{"SCHEMA_ERROR", "invalid if", ""}
			}
			c, err := e.Expr(x.Cond, env)
			if err != nil {
				return nil, false, err
			}
			ok, e2 := truth(c)
			if e2 != nil {
				return nil, false, e2
			}
			if ok {
				v, r, er := e.execBlock(x.Then, env)
				return v, r, er
			}
			v, r, er := e.execBlock(x.Else, env)
			return v, r, er
		case "while":
			var x struct {
				Condition json.RawMessage `json:"condition"`
				Body      []ast.Stmt
				Max       int `json:"max"`
			}
			if err := json.Unmarshal(b, &x); err != nil {
				return nil, false, Error{"SCHEMA_ERROR", "invalid while", ""}
			}
			if x.Max <= 0 {
				x.Max = 10000
			}
			for i := 0; i < x.Max; i++ {
				c, er := e.Expr(x.Condition, env)
				if er != nil {
					return nil, false, er
				}
				ok, er := truth(c)
				if er != nil {
					return nil, false, er
				}
				if !ok {
					break
				}
				v, r, er := e.execBlock(x.Body, env)
				if er != nil || r {
					return v, r, er
				}
			}
			return nil, false, nil
		case "for":
			var x struct {
				Name string          `json:"name"`
				List json.RawMessage `json:"list"`
				Body []ast.Stmt
			}
			if err := json.Unmarshal(b, &x); err != nil {
				return nil, false, Error{"SCHEMA_ERROR", "invalid for", ""}
			}
			v, er := e.Expr(x.List, env)
			if er != nil {
				return nil, false, er
			}
			a, ok := v.([]interface{})
			if !ok {
				return nil, false, Error{"RUNTIME_ERROR", "for requires a list", ""}
			}
			for _, item := range a {
				env[x.Name] = item
				v, r, er := e.execBlock(x.Body, env)
				if er != nil || r {
					return v, r, er
				}
			}
		case "function":
			var x struct {
				Name   string     `json:"name"`
				Params []string   `json:"params"`
				Body   []ast.Stmt `json:"body"`
			}
			if err := json.Unmarshal(b, &x); err != nil || x.Name == "" {
				return nil, false, Error{"SCHEMA_ERROR", "invalid function", ""}
			}
			bb, _ := json.Marshal(x.Body)
			e.Functions[x.Name] = map[string]interface{}{"params": x.Params, "body": bb}
		case "try":
			var x struct {
				Body, Catch []ast.Stmt
				Error       string `json:"error"`
			}
			if err := json.Unmarshal(b, &x); err != nil {
				return nil, false, Error{"SCHEMA_ERROR", "invalid try", ""}
			}
			v, r, er := e.execBlock(x.Body, env)
			if er != nil {
				if x.Error != "" {
					env[x.Error] = er.Error()
				}
				v, r, er = e.execBlock(x.Catch, env)
			}
			return v, r, er
		case "import":
			var path string
			if err := json.Unmarshal(b, &path); err != nil {
				return nil, false, Error{"SCHEMA_ERROR", "import path must be string", ""}
			}
			p, er := loadModule(path, e.cwd)
			if er != nil {
				return nil, false, er
			}
			if e.Imports[path] == 1 {
				return nil, false, Error{"SEMANTIC_ERROR", "circular import: " + path, ""}
			}
			if e.Imports[path] == 2 {
				return nil, false, Error{"SEMANTIC_ERROR", "duplicate import: " + path, ""}
			}
			e.Imports[path] = 1
			_, er = e.Run(p)
			e.Imports[path] = 2
			if er != nil {
				return nil, false, er
			}
		case "prompt":
			var x struct {
				Name     string `json:"name"`
				Template string `json:"template"`
			}
			if err := json.Unmarshal(b, &x); err != nil || x.Name == "" {
				return nil, false, Error{"SCHEMA_ERROR", "invalid prompt", ""}
			}
			env[x.Name] = Prompt(x.Template)
		case "memory":
			var x struct {
				Name string `json:"name"`
				File string `json:"file"`
			}
			if err := json.Unmarshal(b, &x); err != nil || x.Name == "" {
				return nil, false, Error{"SCHEMA_ERROR", "invalid memory", ""}
			}
			env[x.Name] = NewMemory(x.File)
		case "context":
			var x struct {
				Name  string `json:"name"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(b, &x); err != nil || x.Name == "" {
				return nil, false, Error{"SCHEMA_ERROR", "invalid context", ""}
			}
			if x.Limit <= 0 {
				x.Limit = 4000
			}
			env[x.Name] = &ContextWindow{Limit: x.Limit}
		case "tool":
			if err := e.declareTool(b, env); err != nil {
				return nil, false, err
			}
		case "agent":
			if err := e.declareAgent(b, env); err != nil {
				return nil, false, err
			}
		case "stream":
			v, err := e.Expr(b, env)
			if err != nil {
				return nil, false, err
			}
			e.Output(v)
		case "export":
			var name string
			if err := json.Unmarshal(b, &name); err != nil {
				return nil, false, Error{"SCHEMA_ERROR", "export requires string", ""}
			}
			e.Exports[name] = true
		}
	}
	return nil, false, nil
}
func (e *Engine) execBlock(s []ast.Stmt, env map[string]interface{}) (interface{}, bool, error) {
	v, err := e.runBlock(s, env)
	return v, false, err
}

type Prompt string

func (p Prompt) Render(env map[string]interface{}) string {
	r := string(p)
	for k, v := range env {
		r = strings.ReplaceAll(r, "{"+k+"}", fmt.Sprint(v))
	}
	return r
}
func (e *Engine) Expr(b json.RawMessage, env map[string]interface{}) (interface{}, error) {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, Error{"SCHEMA_ERROR", "invalid expression", ""}
	}
	return e.eval(v, env)
}
func (e *Engine) eval(v interface{}, env map[string]interface{}) (interface{}, error) {
	switch x := v.(type) {
	case nil, bool, string, float64:
		return x, nil
	case []interface{}:
		r := make([]interface{}, len(x))
		for i, z := range x {
			q, er := e.eval(z, env)
			if er != nil {
				return nil, er
			}
			r[i] = q
		}
		return r, nil
	case map[string]interface{}:
		if len(x) != 1 {
			return nil, Error{"SCHEMA_ERROR", "expression node must have one key", ""}
		}
		for k, z := range x {
			switch k {
			case "literal":
				return z, nil
			case "var":
				n, ok := z.(string)
				if !ok {
					return nil, Error{"SCHEMA_ERROR", "var name must be string", ""}
				}
				q, ok := env[n]
				if !ok {
					return nil, Error{"RUNTIME_ERROR", "undefined variable: " + n, ""}
				}
				return q, nil
			case "list":
				return e.eval(z, env)
			case "map":
				return e.evalMap(z, env)
			case "binary":
				return e.binary(z, env)
			case "unary":
				return e.unary(z, env)
			case "call":
				return e.call(z, env)
			case "index":
				return e.index(z, env)
			case "property":
				return e.property(z, env)
			}
		}
	}
	return nil, Error{"RUNTIME_ERROR", "unsupported value", ""}
}
func (e *Engine) evalMap(z interface{}, env map[string]interface{}) (interface{}, error) {
	m, ok := z.(map[string]interface{})
	if !ok {
		return nil, Error{"SCHEMA_ERROR", "map expression must be object", ""}
	}
	r := map[string]interface{}{}
	for k, v := range m {
		q, er := e.eval(v, env)
		if er != nil {
			return nil, er
		}
		r[k] = q
	}
	return r, nil
}
func (e *Engine) binary(z interface{}, env map[string]interface{}) (interface{}, error) {
	m, ok := z.(map[string]interface{})
	if !ok {
		return nil, Error{"SCHEMA_ERROR", "binary must be object", ""}
	}
	op, _ := m["op"].(string)
	a, er := e.eval(m["left"], env)
	if er != nil {
		return nil, er
	}
	if op == "and" {
		x, er := truth(a)
		if er != nil {
			return nil, er
		}
		if !x {
			return false, nil
		}
		b, er := e.eval(m["right"], env)
		if er != nil {
			return nil, er
		}
		return truth(b)
	}
	if op == "or" {
		x, er := truth(a)
		if er != nil {
			return nil, er
		}
		if x {
			return true, nil
		}
		b, er := e.eval(m["right"], env)
		if er != nil {
			return nil, er
		}
		return truth(b)
	}
	b, er := e.eval(m["right"], env)
	if er != nil {
		return nil, er
	}
	switch op {
	case "==":
		return reflect.DeepEqual(a, b), nil
	case "!=":
		return !reflect.DeepEqual(a, b), nil
	}
	af, aok := num(a)
	bf, bok := num(b)
	if aok && bok {
		switch op {
		case "+":
			return af + bf, nil
		case "-":
			return af - bf, nil
		case "*":
			return af * bf, nil
		case "/":
			if bf == 0 {
				return nil, Error{"RUNTIME_ERROR", "division by zero", ""}
			}
			return af / bf, nil
		case "%":
			if bf == 0 {
				return nil, Error{"RUNTIME_ERROR", "division by zero", ""}
			}
			return float64(int64(af) % int64(bf)), nil
		case "<":
			return af < bf, nil
		case "<=":
			return af <= bf, nil
		case ">":
			return af > bf, nil
		case ">=":
			return af >= bf, nil
		}
	}
	if op == "+" {
		as, aok := a.(string)
		bs, bok := b.(string)
		if aok && bok {
			return as + bs, nil
		}
	}
	return nil, Error{"RUNTIME_ERROR", "invalid operands for " + op, ""}
}
func (e *Engine) unary(z interface{}, env map[string]interface{}) (interface{}, error) {
	m, ok := z.(map[string]interface{})
	if !ok {
		return nil, Error{"SCHEMA_ERROR", "unary must be object", ""}
	}
	op, _ := m["op"].(string)
	a, er := e.eval(m["value"], env)
	if er != nil {
		return nil, er
	}
	switch op {
	case "not":
		b, er := truth(a)
		if er != nil {
			return nil, er
		}
		return !b, nil
	case "-":
		n, ok := num(a)
		if !ok {
			return nil, Error{"RUNTIME_ERROR", "unary - requires number", ""}
		}
		return -n, nil
	}
	return nil, Error{"RUNTIME_ERROR", "unknown unary operator: " + op, ""}
}
func (e *Engine) call(z interface{}, env map[string]interface{}) (interface{}, error) {
	m, ok := z.(map[string]interface{})
	if !ok {
		return nil, Error{"SCHEMA_ERROR", "call must be object", ""}
	}
	name, _ := m["name"].(string)
	aa, _ := m["args"].([]interface{})
	args := make([]interface{}, len(aa))
	for i, v := range aa {
		q, er := e.eval(v, env)
		if er != nil {
			return nil, er
		}
		args[i] = q
	}
	switch name {
	case "print":
		for _, v := range args {
			e.Output(v)
		}
		return nil, nil
	case "prompt.render":
		if len(args) < 1 {
			return nil, Error{"RUNTIME_ERROR", "prompt.render requires prompt", ""}
		}
		pr, ok := args[0].(Prompt)
		if !ok {
			return nil, Error{"RUNTIME_ERROR", "first argument is not prompt", ""}
		}
		vals := map[string]interface{}{}
		if len(args) > 1 {
			if mm, ok := args[1].(map[string]interface{}); ok {
				vals = mm
			}
		}
		return pr.Render(vals), nil
	case "llm":
		if len(args) < 1 {
			return nil, Error{"RUNTIME_ERROR", "llm requires prompt", ""}
		}
		prompt := fmt.Sprint(args[0])
		model := ""
		temp := 0.0
		if len(args) > 1 {
			model = fmt.Sprint(args[1])
		}
		if len(args) > 2 {
			temp, _ = num(args[2])
		}
		res, er := e.Provider.Complete(context.Background(), providers.Request{Prompt: prompt, Model: model, Temperature: temp})
		if er != nil {
			return nil, er
		}
		if out, ok := m["output"]; ok {
			if sm, ok := out.(map[string]interface{}); ok {
				return Structured(res, sm)
			}
		}
		return res, nil
	case "memory.set":
		if len(args) != 3 {
			return nil, Error{"RUNTIME_ERROR", "memory.set(memory,key,value) required", ""}
		}
		mm, ok := args[0].(*Memory)
		if !ok {
			return nil, Error{"RUNTIME_ERROR", "first argument is not memory", ""}
		}
		key := fmt.Sprint(args[1])
		mm.Data[key] = args[2]
		return mm.Save(), nil
	case "memory.get":
		mm, ok := args[0].(*Memory)
		if !ok {
			return nil, Error{"RUNTIME_ERROR", "first argument is not memory", ""}
		}
		return mm.Data[fmt.Sprint(args[1])], nil
	case "memory.delete":
		mm, ok := args[0].(*Memory)
		if !ok {
			return nil, Error{"RUNTIME_ERROR", "first argument is not memory", ""}
		}
		delete(mm.Data, fmt.Sprint(args[1]))
		return mm.Save(), nil
	case "memory.contains":
		mm, ok := args[0].(*Memory)
		if !ok {
			return nil, Error{"RUNTIME_ERROR", "first argument is not memory", ""}
		}
		_, ok = mm.Data[fmt.Sprint(args[1])]
		return ok, nil
	case "memory.clear":
		mm, ok := args[0].(*Memory)
		if !ok {
			return nil, Error{"RUNTIME_ERROR", "first argument is not memory", ""}
		}
		mm.Data = map[string]interface{}{}
		return mm.Save(), nil
	case "context.add":
		c, ok := args[0].(*ContextWindow)
		if !ok {
			return nil, Error{"RUNTIME_ERROR", "not a context", ""}
		}
		c.Add(fmt.Sprint(args[1]))
		return nil, nil
	case "context.text":
		c, ok := args[0].(*ContextWindow)
		if !ok {
			return nil, Error{"RUNTIME_ERROR", "not a context", ""}
		}
		return c.Text(), nil
	case "llm.stream":
		if len(args) < 1 {
			return nil, Error{"RUNTIME_ERROR", "llm.stream requires prompt", ""}
		}
		ch, er := e.Provider.Stream(context.Background(), providers.Request{Prompt: fmt.Sprint(args[0])})
		var b strings.Builder
		for x := range ch {
			e.Output(x)
			b.WriteString(x)
			b.WriteByte(' ')
		}
		if x := <-er; x != nil {
			return nil, x
		}
		return strings.TrimSpace(b.String()), nil
	case "tool.call":
		return e.toolCall(args)
	case "agent.run":
		return e.agentRun(args)
	}
	f, ok := e.Functions[name]
	if !ok {
		return nil, Error{"RUNTIME_ERROR", "unknown function: " + name, ""}
	}
	params, _ := f["params"].([]string)
	body, _ := f["body"].([]ast.Stmt)
	local := map[string]interface{}{}
	for i, p := range params {
		if i < len(args) {
			local[p] = args[i]
		} else {
			return nil, Error{"RUNTIME_ERROR", "missing function argument", ""}
		}
	}
	return e.runBlock(body, local)
}
func (e *Engine) index(z interface{}, env map[string]interface{}) (interface{}, error) {
	m := z.(map[string]interface{})
	a, er := e.eval(m["target"], env)
	if er != nil {
		return nil, er
	}
	i, er := e.eval(m["index"], env)
	if er != nil {
		return nil, er
	}
	if ar, ok := a.([]interface{}); ok {
		n, no := num(i)
		if !no || n < 0 || int(n) >= len(ar) {
			return nil, Error{"RUNTIME_ERROR", "index out of range", ""}
		}
		return ar[int(n)], nil
	}
	if mm, ok := a.(map[string]interface{}); ok {
		k, ko := i.(string)
		if !ko {
			return nil, Error{"RUNTIME_ERROR", "map index requires string", ""}
		}
		return mm[k], nil
	}
	return nil, Error{"RUNTIME_ERROR", "value is not indexable", ""}
}
func (e *Engine) property(z interface{}, env map[string]interface{}) (interface{}, error) {
	m := z.(map[string]interface{})
	a, er := e.eval(m["target"], env)
	if er != nil {
		return nil, er
	}
	p, _ := m["name"].(string)
	if mm, ok := a.(map[string]interface{}); ok {
		return mm[p], nil
	}
	return nil, Error{"RUNTIME_ERROR", "property access requires map", ""}
}
func (e *Engine) toolCall(a []interface{}) (interface{}, error) {
	if len(a) < 2 {
		return nil, Error{"RUNTIME_ERROR", "tool.call(name,input) required", ""}
	}
	name := fmt.Sprint(a[0])
	t, ok := e.Tools[name]
	if !ok {
		return nil, Error{"RUNTIME_ERROR", "unknown tool: " + name, ""}
	}
	in, ok := a[1].(map[string]interface{})
	if !ok {
		return nil, Error{"RUNTIME_ERROR", "tool input must be map", ""}
	}
	if er := validateSchema(t.Schema, in); er != nil {
		return nil, er
	}
	if t.Impl == nil {
		return nil, Error{"RUNTIME_ERROR", "tool has no implementation", ""}
	}
	return t.Impl(in)
}
func (e *Engine) agentRun(a []interface{}) (interface{}, error) {
	if len(a) < 2 {
		return nil, Error{"RUNTIME_ERROR", "agent.run(name,prompt) required", ""}
	}
	name := fmt.Sprint(a[0])
	ag, ok := e.Agents[name]
	if !ok {
		return nil, Error{"RUNTIME_ERROR", "unknown agent: " + name, ""}
	}
	if ag.Steps >= ag.MaxSteps {
		return nil, Error{"AI_ERROR", "agent step limit exceeded", ""}
	}
	ag.Steps++
	e.Agents[name] = ag
	return ag.Provider.Complete(context.Background(), providers.Request{Prompt: ag.Instructions + "\n" + fmt.Sprint(a[1]), Model: ag.Model})
}
func (e *Engine) declareTool(b json.RawMessage, env map[string]interface{}) error {
	var x struct {
		Name, Description string
		Schema            map[string]interface{}
		Implementation    string `json:"implementation"`
	}
	if er := json.Unmarshal(b, &x); er != nil || x.Name == "" {
		return Error{"SCHEMA_ERROR", "invalid tool", ""}
	}
	impl := func(in map[string]interface{}) (interface{}, error) {
		if x.Implementation == "echo" {
			return in, nil
		}
		return nil, Error{"RUNTIME_ERROR", "tool implementation must be echo or use host registration", ""}
	}
	e.Tools[x.Name] = Tool{x.Name, x.Description, x.Schema, impl}
	return nil
}
func (e *Engine) declareAgent(b json.RawMessage, env map[string]interface{}) error {
	var x struct {
		Name, Instructions string
		Tools              []string
		Memory             string
		Model              string
		MaxSteps           int `json:"max_steps"`
	}
	if er := json.Unmarshal(b, &x); er != nil || x.Name == "" {
		return Error{"SCHEMA_ERROR", "invalid agent", ""}
	}
	if x.MaxSteps <= 0 {
		x.MaxSteps = 8
	}
	ag := Agent{Name: x.Name, Instructions: x.Instructions, Tools: map[string]Tool{}, Memory: e.Memory, Provider: e.Provider, Model: x.Model, MaxSteps: x.MaxSteps}
	for _, n := range x.Tools {
		if t, ok := e.Tools[n]; ok {
			ag.Tools[n] = t
		} else {
			return Error{"SEMANTIC_ERROR", "agent references unknown tool: " + n, ""}
		}
	}
	e.Agents[x.Name] = ag
	return nil
}
func validateSchema(s map[string]interface{}, in map[string]interface{}) error {
	if s == nil {
		return nil
	}
	req, _ := s["required"].([]interface{})
	for _, x := range req {
		n, _ := x.(string)
		if _, ok := in[n]; !ok {
			return Error{"RUNTIME_ERROR", "missing required tool input: " + n, ""}
		}
	}
	props, _ := s["properties"].(map[string]interface{})
	for n, v := range in {
		if p, ok := props[n].(map[string]interface{}); ok {
			typ, _ := p["type"].(string)
			if typ != "" && !typeOK(v, typ) {
				return Error{"RUNTIME_ERROR", "invalid type for tool input: " + n, ""}
			}
		}
	}
	return nil
}
func typeOK(v interface{}, t string) bool {
	switch t {
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		_, ok := num(v)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "object":
		_, ok := v.(map[string]interface{})
		return ok
	case "array":
		_, ok := v.([]interface{})
		return ok
	case "null":
		return v == nil
	}
	return false
}
func truth(v interface{}) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, Error{"RUNTIME_ERROR", "condition requires bool", ""}
	}
	return b, nil
}
func num(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}
func loadModule(path, cwd string) (ast.Program, error) {
	if !strings.HasSuffix(path, ".ail") {
		path += ".ail"
	}
	if !strings.HasPrefix(path, "/") {
		path = cwd + "/" + path
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return ast.Program{}, Error{"RUNTIME_ERROR", "module not found: " + path, ""}
	}
	return ast.Parse(b)
}
func Structured(raw string, schema map[string]interface{}) (interface{}, error) {
	var v interface{}
	if e := json.Unmarshal([]byte(raw), &v); e != nil {
		return nil, Error{"AI_ERROR", "structured output is not valid JSON", ""}
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, Error{"AI_ERROR", "structured output must be JSON object", ""}
	}
	if e := validateSchema(schema, m); e != nil {
		return nil, Error{"AI_ERROR", e.Error(), ""}
	}
	return v, nil
}
func _() { _ = strconv.Itoa; _ = errors.New }
