package logger

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogSetup_DebugFlagTrue(t *testing.T) {
	LogSetup(true)
	assert.Equal(t, logrus.DebugLevel, L.Level)
}
