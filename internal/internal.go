package internal

import (
	"github.com/jesusprubio/up/pkg"
)

const (
	AppName  = "upd"
	AppShort = "Tool to monitor if the network connection is up."
	AppDesc  = `
	Runs HTTP, TCP or DNS checks on a regular basis.
    If all checks fail, runs down action on a regular basis until the
    connection is back up.
	`
)

// ProtocolByID returns the protocol implementation whose ID matches the given
// one.
// Lifted from https://github.com/jesusprubio/up - Copyright Jesús Rubio
// <jesusprubio@gmail.com>
func ProtocolByID(id string) *pkg.Protocol {
	if id == "https" {
		id = "http"
	}
	for _, p := range pkg.Protocols {
		if p.ID == id {
			return p
		}
	}
	return nil
}
