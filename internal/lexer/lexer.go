package lexer

// Token is retained as a tiny internal utility for REPL diagnostics. Ail source is JSON in v1.
type Token struct {
	Kind, Text   string
	Line, Column int
}
