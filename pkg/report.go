// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package pkg

import (
	"time"
)

// Report is the result of a connection attempt.
//
// Only one of the properties 'Response' or 'Error' is set.
type Report struct {
	// Protocol used to connect to.
	ProtocolID string `json:"protocol"`
	// Target used to connect to.
	RHost string `json:"rhost"`
	// Response time.
	Time time.Duration `json:"time"`
	// Extra information. Depending on the protocol, it could be:
	// - HTTP: Response code.
	// - TCP: Local address.
	// - DNS: Resolved IP addresses.
	Extra string `json:"extra,omitempty"`
	// Network error.
	Error error `json:"error,omitempty"`
}
