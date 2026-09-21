# Ail 1.0.0

Ail is a small AI-native enterprise programming language whose canonical program form is strict JSON. An AI Engineer may translate English requirements into Ail JSON and diagnose or patch Ail JSON, while the compiler and validator remain deterministic authorities.

## Architecture

`English → AI Engineer → Ail JSON → deterministic Ail runtime/compiler → Go / Java`

Ail JSON is the source of truth. v1 uses no alternate source syntax. The runtime provides ordinary language features plus bounded AI primitives: prompts, LLM calls, memory, context, streams, tools, and agents.

## Quick start

Requires Go 1.22+.

```sh
go test ./...
go build -o ail ./cmd/ail
./ail version
./ail run examples/hello.ail
```

## CLI

- `ail run file.ail`
- `ail test`
- `ail repl`
- `ail version`
- `ail help`

The REPL accepts JSON expressions and supports `exit`/`quit` and EOF.

## Providers

Without `OPENAI_API_KEY`, the CLI uses the deterministic MockProvider. Set `OPENAI_API_KEY` to select the OpenAI provider. Optional `OPENAI_BASE_URL` overrides the default `https://api.openai.com/v1`. Ail never embeds or prints credentials.

The OpenAI provider uses the standard library HTTP client and supports normal chat requests and SSE streaming. Structured output is validated locally after parsing.

## Examples

See `examples/hello.ail`, `prompt.ail`, `agent.ail`, `memory.ail`, and `stream.ail`. They require no network and use MockProvider when no API key is configured.

## Testing and building

`go test ./...` is the complete Go test suite. `scripts/test.sh` runs it. `scripts/build.sh` builds the CLI. `scripts/run_examples.sh` executes all five examples.

Cross compilation needs only the Go toolchain, for example:

```sh
GOOS=linux GOARCH=amd64 go build -o ail-linux-amd64 ./cmd/ail
GOOS=darwin GOARCH=arm64 go build -o ail-darwin-arm64 ./cmd/ail
GOOS=windows GOARCH=amd64 go build -o ail.exe ./cmd/ail
```

`CGO_ENABLED=0 go build -o ail ./cmd/ail` is the supported static-build command on platforms where Go permits it.

## Targets

The library exposes deterministic `CompileGo` and `CompileJava` functions. They emit a deliberately small supported subset from Ail rather than pretending arbitrary Go or Java can be reverse engineered from every Ail feature. The generated output contains no AI step.

## Project structure

- `cmd/ail`: executable
- `internal/ast`: canonical JSON schema and representation
- `internal/parser`, `internal/lexer`: JSON parsing and small token utility
- `internal/evaluator`: language runtime and AI-native primitives
- `internal/providers`: MockProvider and OpenAIProvider
- `internal`: deterministic Go/Java target emitters
- `examples`, `tests`, `stdlib`, `scripts`: shipped programs, tests, library, and build helpers

## Limitations

v1 intentionally has no package manager, classes, generics, macros, GUI, database layer, hidden process execution, or unrestricted autonomous agent loop. Tool implementations supplied in JSON are intentionally limited to deterministic `echo`; host applications can use the evaluator's Tool type for explicit native implementations.

## Stability

Ail 1.0.0 is frozen except for reproducible bugs, security issues, specification violations, or broken build/test infrastructure. Semantic changes are not silently introduced.
