package internal

import (
	"fmt"
	"os"
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

func readTestConfig(cfgFile string) (*Configuration, error) {
	return ReadConf(fmt.Sprintf("%s/%s", testConfigDir, cfgFile))
}

func (suite *TestSuite) SetupTest() {
	nulllogger.NewNullLoggerHook()
	var err error
	suite.conf, err = readTestConfig("upd_test_good.yaml")
	assert.Nil(suite.T(), err, "No error expected")
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
	conf, err := readTestConfig("upd_test_noda.yaml")
	assert.Nil(t, err, "No error expected")
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
	conf, err := readTestConfig("upd_test_bad.yaml")
	assert.Nil(t, err, "No error expected")
	checklist, checkErr := conf.GetChecks()
	assert.Nil(t, checkErr, "No error expected")
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
	nulllogger.NewNullLoggerHook()
	logger.L.ExitFunc = func(code int) { panic(code) }
	conf, err := readTestConfig("upd_test_allbad.yaml")
	assert.Nil(t, err, "No error expected")
	_, checkErr := conf.GetChecks()
	assert.ErrorIs(t, checkErr, ErrNoChecks, "Error expected: no valid checks")
}

func (suite *TestSuite) TestGetChecks() {
	checklist, checkErr := suite.conf.GetChecks()
	assert.Nil(suite.T(), checkErr, "No error expected")
	// Collect all checks from both Ordered and Shuffled
	allChecks := append([]*pkg.Check{}, checklist.Ordered...)
	allChecks = append(allChecks, checklist.Shuffled...)
	var probe pkg.Probe
	var http *pkg.HTTPProbe
	var dns *pkg.DNSProbe
	var tcp *pkg.TCPProbe
	var ok bool
	assert.Equal(suite.T(), 4, len(allChecks))
	probe = *allChecks[0].Probe
	http, ok = probe.(*pkg.HTTPProbe)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "http", http.Scheme())
	assert.Equal(suite.T(), "http://captive.apple.com/hotspot-detect.html", http.URL)
	probe = *allChecks[1].Probe
	http, ok = probe.(*pkg.HTTPProbe)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "http", http.Scheme())
	assert.Equal(suite.T(), "https://example.com/", http.URL)
	probe = *allChecks[2].Probe
	dns, ok = probe.(*pkg.DNSProbe)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "dns", dns.Scheme())
	assert.Equal(suite.T(), "1.1.1.1:53", dns.DNSResolver)
	assert.Equal(suite.T(), "www.google.com", dns.Domain)
	probe = *allChecks[3].Probe
	tcp, ok = probe.(*pkg.TCPProbe)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "tcp", tcp.Scheme())
	assert.Equal(suite.T(), "1.0.0.1:53", tcp.HostPort)
}

func (suite *TestSuite) TestStatConf() {
	conf := suite.conf.Stats
	assert.Equal(suite.T(), ":8080", conf.Port)
}

func TestReadConf_envsubst(t *testing.T) {
	os.Setenv("UPD_TEST_TIMEOUT", "3s")
	defer os.Unsetenv("UPD_TEST_TIMEOUT")

	conf, err := readTestConfig("upd_test_envvar.yaml")
	assert.NoError(t, err)
	assert.Equal(t, 3*time.Second, conf.Checks.TimeOut)
}

func TestReadConf_envsubst_missing(t *testing.T) {
	_, err := readTestConfig("upd_test_envvar.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TimeOut")
}

func TestDNSCheckValidation_MissingDomain(t *testing.T) {
	nulllogger.NewNullLoggerHook()
	conf, err := readTestConfig("upd_test_bad.yaml")
	assert.Nil(t, err, "No error expected")
	checklist, checkErr := conf.GetChecks()
	assert.Nil(t, checkErr, "No error expected")

	// Check that dns://8.8.4.4/ is ignored due to missing domain
	dnsChecks := 0
	for _, check := range checklist.Ordered {
		_, ok := (*check.Probe).(*pkg.DNSProbe)
		if ok {
			dnsChecks++
		}
	}
	for _, check := range checklist.Shuffled {
		_, ok := (*check.Probe).(*pkg.DNSProbe)
		if ok {
			dnsChecks++
		}
	}
	assert.Equal(t, 0, dnsChecks, "DNS check with missing domain should be ignored")
}

func TestDNSCheckValidation_MissingResolver(t *testing.T) {
	nulllogger.NewNullLoggerHook()
	conf, err := readTestConfig("upd_test_dns_missing_resolver.yaml")
	assert.NoError(t, err)

	checklist, checkErr := conf.GetChecks()
	assert.Nil(t, checkErr, "No error expected")

	// Check that dns:///google.com is ignored due to missing resolver host
	dnsChecks := 0
	for _, check := range checklist.Ordered {
		_, ok := (*check.Probe).(*pkg.DNSProbe)
		if ok {
			dnsChecks++
		}
	}
	for _, check := range checklist.Shuffled {
		_, ok := (*check.Probe).(*pkg.DNSProbe)
		if ok {
			dnsChecks++
		}
	}
	assert.Equal(t, 0, dnsChecks, "DNS check with missing resolver host should be ignored")

	// Verify we still have the HTTP check
	httpChecks := 0
	for _, check := range checklist.Ordered {
		_, ok := (*check.Probe).(*pkg.HTTPProbe)
		if ok {
			httpChecks++
		}
	}
	for _, check := range checklist.Shuffled {
		_, ok := (*check.Probe).(*pkg.HTTPProbe)
		if ok {
			httpChecks++
		}
	}
	assert.Equal(t, 1, httpChecks, "HTTP check should still be present")
}

func TestLogSetup(t *testing.T) {
	tests := []struct {
		name      string
		logLevel  string
		wantLevel string
	}{
		{"trace level", "trace", "trace"},
		{"debug level", "debug", "debug"},
		{"info level", "info", "info"},
		{"warn level", "warn", "warning"},
		{"empty defaults to warn", "", "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nulllogger.NewNullLoggerHook()
			conf := &Configuration{LogLevel: tt.logLevel}
			conf.logSetup()
			assert.Equal(t, tt.wantLevel, logger.L.Level.String())
		})
	}
}
