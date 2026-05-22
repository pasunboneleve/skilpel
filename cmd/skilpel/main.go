package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pasunboneleve/skilpel/internal/skilpel"
)

func main() {
	code, err := skilpel.Main(context.Background(), os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	os.Exit(code)
}
