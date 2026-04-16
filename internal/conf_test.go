package internal

import (
	"fmt"
	"testing"
	"time"

	"github.com/hugoh/upd/internal/check"
	"github.com/hugoh/upd/internal/logic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite

	conf *Configuration
}

const testConfigDir = "../testdata"

func readTestConfig(cfgFile string) (*Configuration, error) {
	return ReadConf(fmt.Sprintf("%s/%s", testConfigDir, cfgFile))
}

func (suite *TestSuite) SetupTest() {
	var err error

	suite.conf, err = readTestConfig("upd_test_good.yaml")
	suite.NoError(err)
}

func TestSuiteRun(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (suite *TestSuite) TestGetDownActionFromConf() {
	da := suite.conf.GetDownAction()
	suite.NotNil(da, "DownAction parsed")
	suite.Equal(&logic.DownAction{
		After: 120 * time.Second,
		Every: 300 * time.Second,
		Exec:  "cowsay",
	}, da)
}

func TestNoDownAction(t *testing.T) {
	conf, err := readTestConfig("upd_test_noda.yaml")
	require.NoError(t, err)

	da := conf.GetDownAction()
	assert.Nil(t, da, "DownAction not found")
}

func (suite *TestSuite) TestGetDelaysFromConf() {
	delays := make(map[bool]time.Duration)
	delays[true] = 120 * time.Second
	delays[false] = 20 * time.Second
	conf := suite.conf.GetDelays()
	suite.Equal(delays, conf)
}

func TestGetChecksIgnored(t *testing.T) {
	conf, err := readTestConfig("upd_test_bad.yaml")
	require.NoError(t, err)

	checklist, checkErr := conf.GetChecks()
	require.NoError(t, checkErr)
	// There should be 1 valid check in total (Ordered + Shuffled)
	// - http://captive.apple.com/hotspot-detect.html is valid
	// - ftp://foo.bar/ is ignored (unknown protocol)
	// - dns://8.8.4.4/ is ignored (missing domain)
	totalChecks := 0
	if checklist != nil {
		totalChecks = len(checklist.Ordered) + len(checklist.Shuffled)
	}

	assert.Equal(t, 1, totalChecks, "2 checks should be invalid")
}

func TestGetChecksFromConfFail(t *testing.T) {
	conf, err := readTestConfig("upd_test_allbad.yaml")
	require.NoError(t, err)

	_, checkErr := conf.GetChecks()
	assert.ErrorIs(t, checkErr, ErrNoChecks, "Error expected: no valid checks")
}

func (suite *TestSuite) TestGetChecks() {
	checklist, err := suite.conf.GetChecks()
	suite.Require().NoError(err)
	// Collect all checks from both Ordered and Shuffled
	allChecks := append([]*check.Check{}, checklist.Ordered...)
	allChecks = append(allChecks, checklist.Shuffled...)

	var (
		probe     check.Probe
		httpProbe *check.HTTPProbe
		dns       *check.DNSProbe
		tcp       *check.TCPProbe
		ok        bool
	)

	suite.Len(allChecks, 4)
	probe = allChecks[0].Probe
	httpProbe, ok = probe.(*check.HTTPProbe)
	suite.True(ok)
	suite.Equal("http", httpProbe.Scheme())
	suite.Equal("http://captive.apple.com/hotspot-detect.html", httpProbe.URL)

	probe = allChecks[1].Probe
	httpProbe, ok = probe.(*check.HTTPProbe)
	suite.True(ok)
	suite.Equal("http", httpProbe.Scheme())
	suite.Equal("https://example.com/", httpProbe.URL)

	probe = allChecks[2].Probe
	dns, ok = probe.(*check.DNSProbe)
	suite.True(ok)
	suite.Equal("dns", dns.Scheme())
	suite.Equal("1.1.1.1:53", dns.DNSResolver)
	suite.Equal("www.google.com", dns.Domain)

	probe = allChecks[3].Probe
	tcp, ok = probe.(*check.TCPProbe)
	suite.True(ok)
	suite.Equal("tcp", tcp.Scheme())
	suite.Equal("1.0.0.1:53", tcp.HostPort)
}

func (suite *TestSuite) TestStatConf() {
	conf := suite.conf.Stats
	suite.Equal(":8080", conf.Port)
}

func TestReadConf_envsubst(t *testing.T) {
	t.Setenv("UPD_TEST_TIMEOUT", "3s")

	conf, err := readTestConfig("upd_test_envvar.yaml")
	require.NoError(t, err)
	assert.Equal(t, 3*time.Second, conf.Checks.TimeOut)
}

func TestReadConf_envsubst_missing(t *testing.T) {
	_, err := readTestConfig("upd_test_envvar.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TimeOut")
}

func TestDNSCheckValidation_MissingDomain(t *testing.T) {
	conf, err := readTestConfig("upd_test_bad.yaml")
	require.NoError(t, err)

	checklist, checkErr := conf.GetChecks()
	require.NoError(t, checkErr)

	// Check that dns://8.8.4.4/ is ignored due to missing domain
	dnsChecks := 0

	for _, chk := range checklist.Ordered {
		_, ok := chk.Probe.(*check.DNSProbe)
		if ok {
			dnsChecks++
		}
	}

	for _, chk := range checklist.Shuffled {
		_, ok := chk.Probe.(*check.DNSProbe)
		if ok {
			dnsChecks++
		}
	}

	assert.Equal(t, 0, dnsChecks, "DNS check with missing domain should be ignored")
}

func TestDNSCheckValidation_MissingResolver(t *testing.T) {
	conf, err := readTestConfig("upd_test_dns_missing_resolver.yaml")
	require.NoError(t, err)

	checklist, checkErr := conf.GetChecks()
	require.NoError(t, checkErr)

	// Check that dns:///google.com is ignored due to missing resolver host
	dnsChecks := 0

	for _, chk := range checklist.Ordered {
		_, ok := chk.Probe.(*check.DNSProbe)
		if ok {
			dnsChecks++
		}
	}

	for _, chk := range checklist.Shuffled {
		_, ok := chk.Probe.(*check.DNSProbe)
		if ok {
			dnsChecks++
		}
	}

	assert.Equal(t, 0, dnsChecks, "DNS check with missing resolver host should be ignored")

	// Verify we still have the HTTP check
	httpChecks := 0

	for _, chk := range checklist.Ordered {
		_, ok := chk.Probe.(*check.HTTPProbe)
		if ok {
			httpChecks++
		}
	}

	for _, chk := range checklist.Shuffled {
		_, ok := chk.Probe.(*check.HTTPProbe)
		if ok {
			httpChecks++
		}
	}

	assert.Equal(t, 1, httpChecks, "HTTP check should still be present")
}

func TestLogSetup(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{"trace level", "trace"},
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"empty defaults to warn", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			conf := &Configuration{LogLevel: tt.logLevel}
			conf.logSetup()
		})
	}
}
