package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	v := Version()
	assert.Equal(t, "dev", v)
}
