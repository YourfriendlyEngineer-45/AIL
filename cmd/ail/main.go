package main

import (
	"ail/internal/cli"
	"os"
)

func main() { os.Exit(cli.Run(os.Args[1:])) }
