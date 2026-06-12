// Package main is the entry point for the upd application.
package main

import (
	"fmt"
	"os"

	"github.com/hugoh/upd/internal"
)

func main() {
	err := internal.Cmd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
