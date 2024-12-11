package internal

import (
	"fmt"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/logic"
	"github.com/hugoh/upd/internal/nulllogger"
	"github.com/hugoh/upd/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	conf *Configuration
}

const testConfigDir = "../testdata"

func readConf(cfgFile string) *Configuration {
	conf := ReadConf(fmt.Sprintf("%s/%s", testConfigDir, cfgFile), false)
	return conf
}

func (suite *TestSuite) SetupTest() {
	nulllogger.NewNullLoggerHook()
	suite.conf = readConf("upd_test_good.yaml")
}

func TestSuiteRun(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) TestGetDownActionFromConf() {
	da := suite.conf.GetDownAction()
	assert.NotNil(suite.T(), da, "DownAction parsed")
	assert.Equal(suite.T(), &logic.DownAction{
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

func TestGetChecksIgnored(t *testing.T) {
	conf := readConf("upd_test_bad.yaml")
	checks := conf.GetChecks()
	assert.Equal(t, 2, len(checks), "1 check is invalid")
}

func TestGetChecksFromConfFail(t *testing.T) {
	nulllogger.NewNullLoggerHook()
	logger.Logger.ExitFunc = func(code int) { panic(code) }
	conf := readConf("upd_test_allbad.yaml")
	assert.Panics(t, func() { conf.GetChecks() })
}

func (suite *TestSuite) TestGetChecks() {
	ret := suite.conf.GetChecks()
	var probe pkg.Probe
	var http *pkg.HTTPProbe
	var dns *pkg.DNSProbe
	var tcp *pkg.TCPProbe
	var ok bool
	assert.Equal(suite.T(), 4, len(ret))
	probe = *ret[0].Probe
	http, ok = probe.(*pkg.HTTPProbe)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "http", http.Scheme())
	assert.Equal(suite.T(), "http://captive.apple.com/hotspot-detect.html", http.URL)
	probe = *ret[1].Probe
	http, ok = probe.(*pkg.HTTPProbe)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "http", http.Scheme())
	assert.Equal(suite.T(), "https://example.com/", http.URL)
	probe = *ret[2].Probe
	dns, ok = probe.(*pkg.DNSProbe)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "dns", dns.Scheme())
	assert.Equal(suite.T(), "1.1.1.1:53", dns.DNSResolver)
	assert.Equal(suite.T(), "www.google.com", dns.Domain)
	probe = *ret[3].Probe
	tcp, ok = probe.(*pkg.TCPProbe)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "tcp", tcp.Scheme())
	assert.Equal(suite.T(), "1.0.0.1:53", tcp.HostPort)
}
