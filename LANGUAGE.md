# Ail Language Specification 1.0

## 1. Program grammar

A program is a JSON object containing exactly `version` and `program`. `version` must be integer `1`; `program` is an array of statement objects. Each statement contains exactly one known node key. Unknown statement keys and unknown fields are schema errors.

Expressions are JSON literals or a single-key expression object. Supported nodes are `literal`, `var`, `binary`, `unary`, `call`, `list`, `map`, `index`, and `property`.

## 2. Values

The runtime represents strings, numbers, booleans, null, lists, maps, functions, prompts, memories, contexts, tools, agents, and streams. JSON numbers are IEEE-754-style Go `float64` values. There is no implicit coercion between unrelated types.

## 3. Variables and scope

`let` creates a variable and rejects duplicate names in the same environment. `set` assigns an existing or new name. Function calls create local lexical environments. A function has named parameters and a statement body and may recursively call itself.

## 4. Expressions

Binary operators: `+ - * / % == != < <= > >= and or`.

Arithmetic requires numbers. `+` also concatenates two strings. Equality uses structural equality. `and` and `or` short-circuit and require boolean operands. Division and modulo by zero are runtime errors. Unary `not` requires bool; unary `-` requires number.

List expressions evaluate each element. Map expressions evaluate each value. Indexing accepts a numeric list index or string map key. Property access is map lookup by string property name.

## 5. Control flow

`if` has `condition`, `then`, optional `else`. Conditions require bool. `while` has `condition`, `body`, optional `max`; its default maximum is 10,000 iterations. `for` iterates a list and binds each item to `name`. `return` optionally returns a value and propagates through the current function.

`try` runs `body`; on error it runs `catch` and may bind the error string to `error`.

## 6. Errors

Errors are categorized as `LEX_ERROR`, `PARSE_ERROR`, `SCHEMA_ERROR`, `SEMANTIC_ERROR`, `RUNTIME_ERROR`, `COMPILE_ERROR`, `TEST_FAILURE`, `AI_ERROR`, and `PROVIDER_ERROR`. The current implementation reports the category and actionable message. Schema and runtime validation never silently accepts invalid operations.

## 7. Modules

`import` accepts a local `.ail` file, adding `.ail` when omitted. Relative modules resolve from the executing file's directory. Missing or invalid modules are errors. v1 does not provide a package manager. Modules execute in the current engine environment. Export declarations record exported names; v1 keeps module resolution intentionally minimal and local.

## 8. Prompts and LLM

`prompt` declares a named `Prompt` value with a template. `prompt.render(prompt, map)` replaces `{name}` placeholders using the supplied map. Replacement is deterministic, literal string replacement; absent placeholders remain unchanged.

`llm(prompt, model?, temperature?)` calls the configured provider. The first argument is converted to its runtime string representation. Providers are responsible for model I/O; the evaluator is responsible for validation and error propagation.

## 9. Structured output

A call may include an `output` schema. The returned provider text must parse as JSON and must satisfy the supported object-schema checks: `required` and `properties.<name>.type` for string, number, boolean, object, array, or null. Invalid JSON or structure is an `AI_ERROR`.

## 10. Tools

A tool has `name`, `description`, `schema`, and `implementation`. JSON tools currently support deterministic `echo`. `tool.call(name,input)` validates required fields and declared primitive property types before execution. Host applications may register native Tool implementations directly through the Go API.

## 11. Agents

An agent has `name`, `instructions`, `tools`, optional memory/model, and `max_steps`. `agent.run(name,prompt)` makes a provider request using the instructions and prompt. Every run increments a step counter and rejects execution after the configured bound. v1 deliberately does not implement an unrestricted autonomous loop or hidden tool-planning loop.

## 12. Memory

Memory is a string-keyed value map with `set`, `get`, `delete`, `contains`, and `clear` builtins. A memory can optionally persist to a JSON file. Saves use normal JSON serialization and file mode 0600. No database is used.

## 13. Context

A context contains ordered strings and a character limit. `context.add` appends content; if the joined content exceeds the limit, oldest entries are discarded until it fits. `context.text` returns the joined content. This is a character-based approximation, not model token counting.

## 14. Streams

`llm.stream(prompt)` consumes the provider stream, emits each received chunk through the runtime output callback, and returns the concatenated text. MockProvider streams whitespace-delimited chunks deterministically. OpenAIProvider consumes Server-Sent Events and forwards non-empty content deltas. Streaming errors are propagated.

## 15. AI Engineer boundary

The AI Engineer is an external capability layer rather than an LLM embedded in the compiler. Its safe lifecycle is read → plan → write/patch Ail JSON → validate → compile → run → test → diagnose → bounded repair. Ail's compiler and validator remain authoritative. A model cannot override a compiler error.

## 16. Determinism and security

JSON serialization uses Go's deterministic map-key ordering. Target generation contains no model calls. Network operations are explicit provider calls. Credentials come only from environment variables. The language does not execute arbitrary model-generated code or provide hidden process execution.

## 17. Target support

The Go and Java emitters intentionally cover a small subset: literal printing, selected simple expressions, and simple `if` emission. They reject invalid program structures before generation. They are deterministic and do not translate arbitrary Go/Java constructs.
