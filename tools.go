//go:build tools

// Package main pins the code-generation tools so `go run` can find them
// without a global install.
package main

import (
	_ "github.com/99designs/gqlgen"
)
