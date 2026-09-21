package cli

import (
	"ail/internal/ast"
	"ail/internal/evaluator"
	"ail/internal/providers"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Version = "1.0.0"

func Run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" {
		help()
		return 0
	}
	switch args[0] {
	case "version":
		fmt.Println("Ail 1.0.0")
		return 0
	case "run":
		if len(args) != 2 {
			return fail("usage: ail run file.ail")
		}
		return runFile(args[1])
	case "test":
		return testDir("tests")
	case "repl":
		return repl()
	}
	return fail("unknown command: " + args[0])
}
func help() {
	fmt.Println("Ail 1.0.0\nUsage: ail <command> [args]\n\nCommands:\n  run <file.ail>  run an Ail JSON program\n  test            run offline Ail test programs\n  repl            start the JSON expression REPL\n  version         print version\n  help            print this help")
}
func runFile(path string) int {
	b, e := os.ReadFile(path)
	if e != nil {
		return fail(e.Error())
	}
	p, e := ast.Parse(b)
	if e != nil {
		return fail(e.Error())
	}
	eng := evaluator.New(provider())
	eng.SetCWD(filepath.Dir(path))
	eng.Output = func(v interface{}) {
		if v != nil {
			fmt.Println(format(v))
		}
	}
	return execute(eng, p, filepath.Dir(path))
}
func execute(e *evaluator.Engine, p ast.Program, cwd string) int { // cwd is intentionally kept for future module resolution; modules are local and deterministic.
	_ = cwd
	_, er := e.Run(p)
	if er != nil {
		fmt.Fprintln(os.Stderr, er)
		return 1
	}
	return 0
}
func provider() providers.Provider {
	if os.Getenv("OPENAI_API_KEY") != "" {
		return providers.NewOpenAI()
	}
	return providers.MockProvider{}
}
func testDir(dir string) int {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return fail(e.Error())
	}
	n := 0
	for _, x := range entries {
		if filepath.Ext(x.Name()) != ".ail" {
			continue
		}
		n++
		b, _ := os.ReadFile(filepath.Join(dir, x.Name()))
		p, e := ast.Parse(b)
		if e != nil {
			fmt.Fprintln(os.Stderr, x.Name(), e)
			return 1
		}
		eng := evaluator.New(providers.MockProvider{})
		if _, e = eng.Run(p); e != nil {
			fmt.Fprintln(os.Stderr, x.Name(), e)
			return 1
		}
		fmt.Println("PASS", x.Name())
	}
	fmt.Printf("%d tests passed\n", n)
	return 0
}
func repl() int {
	fmt.Println("Ail 1.0.0 REPL. Enter JSON expressions or 'exit'.")
	sc := bufio.NewScanner(os.Stdin)
	eng := evaluator.New(providers.MockProvider{})
	env := map[string]interface{}{}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "exit" || line == "quit" {
			break
		}
		if line == "" {
			continue
		}
		var raw json.RawMessage
		if json.Unmarshal([]byte(line), &raw) != nil {
			fmt.Println("PARSE_ERROR: enter a JSON expression")
			continue
		}
		v, e := eng.Expr(raw, env)
		if e != nil {
			fmt.Println(e)
		} else {
			fmt.Println(format(v))
		}
	}
	return 0
}
func format(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
func fail(s string) int { fmt.Fprintln(os.Stderr, s); return 2 }
