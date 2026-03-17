// Package main is the entry point for the upd application.
package main

import (
	"os"

	"github.com/hugoh/upd/internal"
)

func main() {
	err := internal.Cmd()
	if err != nil {
		os.Exit(1)
	}
}
