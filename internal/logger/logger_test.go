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

func TestLogSetup_DebugFlagFalse(t *testing.T) {
	L.SetLevel(logrus.DebugLevel)
	LogSetup(false)
	assert.Equal(t, logrus.DebugLevel, L.Level)
}
