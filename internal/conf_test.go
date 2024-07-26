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
	conf *Configuration
}

const testConfigDir = "../testdata"

func readConf(cfgFile string) *Configuration {
	conf, err := ReadConf(fmt.Sprintf("%s/%s", testConfigDir, cfgFile))
	if err != nil {
		panic(err)
	}
	return conf
}

func (suite *TestSuite) SetupTest() {
	suite.conf = readConf("upd_test_good.yaml")
}

func TestSuiteRun(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) TestGetDownActionFromConf() {
	da, err := suite.conf.GetDownAction()
	assert.NotNil(suite.T(), da, "DownAction parsed")
	assert.NoError(suite.T(), err, "No error while parsing DownAction")
	assert.Equal(suite.T(), &DownAction{
		After: 120 * time.Second,
		Every: 300 * time.Second,
		Exec:  "cowsay",
	}, da)
}

func TestNoDownAction(t *testing.T) {
	conf := readConf("upd_test_noda.yaml")
	da, err := conf.GetDownAction()
	assert.Nil(t, da, "DownAction not found")
	assert.Equal(t, ErrNoDownActionInConf, err)
}

func (suite *TestSuite) TestGetDelaysFromConf() {
	delays := make(map[bool]time.Duration)
	delays[true] = 120 * time.Second
	delays[false] = 20 * time.Second
	conf, err := suite.conf.GetDelays()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), delays, conf)
}

func TestGetChecksFromConfFail(t *testing.T) {
	conf := readConf("upd_test_bad.yaml")
	_, err := conf.GetChecks()
	assert.Error(t, err)
}
