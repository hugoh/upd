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
	conf := ReadConf(fmt.Sprintf("%s/%s", testConfigDir, cfgFile))
	return conf
}

func (suite *TestSuite) SetupTest() {
	suite.conf = readConf("upd_test_good.yaml")
}

func TestSuiteRun(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) TestGetDownActionFromConf() {
	da := suite.conf.GetDownAction()
	assert.NotNil(suite.T(), da, "DownAction parsed")
	assert.Equal(suite.T(), &DownAction{
		After: 120 * time.Second,
		Every: 300 * time.Second,
		Exec:  "cowsay",
	}, da)
}

func TestNoDownAction(t *testing.T) {
	conf := readConf("upd_test_noda.yaml")
	da := conf.GetDownAction()
	assert.Nil(t, da, "DownAction not found")
}

func (suite *TestSuite) TestGetDelaysFromConf() {
	delays := make(map[bool]time.Duration)
	delays[true] = 120 * time.Second
	delays[false] = 20 * time.Second
	conf := suite.conf.GetDelays()
	assert.Equal(suite.T(), delays, conf)
}

// FIXME: need testing of bad config
// func TestGetChecksFromConfFail(t *testing.T) {
// 	conf := readConf("upd_test_bad.yaml")
// 	_, err := conf.GetChecks()
// 	assert.Error(t, err)
// }

func (suite *TestSuite) TestGetChecks() {
	ret := suite.conf.GetChecks()
	assert.Equal(suite.T(), 4, len(ret))
	assert.Equal(suite.T(), "http://captive.apple.com/hotspot-detect.html", ret[0].Target)
	assert.Equal(suite.T(), "https://example.com/", ret[1].Target)
	assert.Equal(suite.T(), "1.1.1.1", ret[2].Target)
	assert.Equal(suite.T(), "1.0.0.1:53", ret[3].Target)
}
