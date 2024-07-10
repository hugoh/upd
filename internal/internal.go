package internal

import (
	"os"

	"github.com/jesusprubio/up/pkg"
	"github.com/sirupsen/logrus"
)

const (
	AppName  = "upd"
	AppShort = "Tool to monitor if the Internet connection is up."
	// FIXME: Update
	AppDesc = `
	@@@
	`
)

// ProtocolByID returns the protocol implementation whose ID matches the given
// one.
// Lifted from https://github.com/jesusprubio/up - Copyright Jesús Rubio
// <jesusprubio@gmail.com>
func ProtocolByID(id string) *pkg.Protocol {
	for _, p := range pkg.Protocols {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// Fatal logs the error to the standard output and exits with status 1.
func FatalIfError(err error) {
	if err == nil {
		return
	}
	logrus.Fatal(err)
	os.Exit(1)
}
