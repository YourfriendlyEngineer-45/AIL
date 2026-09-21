package parser

import (
	"ail/internal/ast"
	"encoding/json"
)

func Parse(data []byte) (ast.Program, error) { return ast.Parse(data) }
func DecodeValue(data []byte) (interface{}, error) {
	var v interface{}
	err := json.Unmarshal(data, &v)
	return v, err
}
