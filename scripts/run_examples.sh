#!/bin/sh
set -eu
for f in examples/hello.ail examples/prompt.ail examples/agent.ail examples/memory.ail examples/stream.ail; do ./ail run "$f"; done
