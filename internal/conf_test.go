package internal

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
}

const testConfigDir = "../test/data"

func readConf(cfgFile string) {
	err := ReadConf(fmt.Sprintf("%s/%s", testConfigDir, cfgFile))
	if err != nil {
		panic(err)
	}
}

func (suite *TestSuite) SetupTest() {
	readConf("upd_test_good.yaml")
}

func TestSuiteRun(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) TestGetDownActionFromConf() {
	da, err := GetDownActionFromConf()
	assert.NotNil(suite.T(), da, "DownAction parsed")
	assert.NoError(suite.T(), err, "No error while parsing DownAction")
	assert.Equal(suite.T(), &DownAction{
		After:    120 * time.Second,
		Every:    300 * time.Second,
		Exec:     "cowsay",
		ExecArgs: []string{},
	}, da)
}

func TestNoDownAction(t *testing.T) {
	readConf("upd_test_noda.yaml")
	da, err := GetDownActionFromConf()
	assert.Nil(t, da, "DownAction not found")
	assert.Equal(t, ErrNoDownActionInConf, err)
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
	readConf("upd_test_bad.yaml")
	_, err := GetChecksFromConf()
	assert.Error(t, err)
}
