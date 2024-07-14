package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
}

func readConf(cfgFile string) {
	err := ReadConf(cfgFile)
	if err != nil {
		panic(err)
	}
}

func (suite *TestSuite) SetupTest() {
	readConf("../test/upd_test_good.yaml")
}

func TestSuiteRun(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) TestGetDownActionFromConf() {
	da, err := GetDownActionFromConf()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), &DownAction{
		After:    120 * time.Second,
		Every:    300 * time.Second,
		Exec:     "cowsay",
		ExecArgs: []string{},
	}, da)
}

func (suite *TestSuite) TestGetDelaysFromConf() {
	delays := make(map[bool]time.Duration)
	delays[true] = 120 * time.Second
	delays[false] = 20 * time.Second
	conf, err := GetDelaysFromConf()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), delays, conf)
}

func TestGetChecksFromConfFail(t *testing.T) {
	readConf("../test/upd_test_bad.yaml")
	_, err := GetChecksFromConf()
	assert.Error(t, err)
}
