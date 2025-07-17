// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package pkg

import (
	"context"
	"time"
)

type Probe interface {
	Probe(ctx context.Context, timeout time.Duration) *Report
	Scheme() string
}
