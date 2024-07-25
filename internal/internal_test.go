package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getProtocol(t *testing.T, input, match string) {
	p := ProtocolByID(input)
	assert.NotNil(t, p)
	assert.Equal(t, match, p.ID, fmt.Sprintf("protocol for %s", input))
}

func TestProtocolById(t *testing.T) {
	getProtocol(t, "http", "http")
	getProtocol(t, "https", "http")
	getProtocol(t, "tcp", "tcp")
	getProtocol(t, "dns", "dns")
	assert.Nil(t, ProtocolByID("foo"))
}
