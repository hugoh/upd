package config

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

// Must match internal/cmd_test.go.
const testConfigDir = "../../testdata"

func readTestConfig(cfgFile string) (*Configuration, error) {
	return ReadConf(fmt.Sprintf("%s/%s", testConfigDir, cfgFile))
}

func (suite *TestSuite) SetupTest() {
	var err error

	suite.conf, err = readTestConfig("upd_test_good.toml")
	suite.Require().NoError(err)
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
	conf, err := readTestConfig("upd_test_noda.toml")
	require.NoError(t, err)

	da := conf.GetDownAction()
	assert.Nil(t, da, "DownAction not found")
}

func (suite *TestSuite) TestGetDelaysFromConf() {
	suite.Equal(logic.Delays{
		Up:   120 * time.Second,
		Down: 20 * time.Second,
	}, suite.conf.GetDelays())
}

func TestReadConf_UnsupportedSchemeFails(t *testing.T) {
	_, err := readTestConfig("upd_test_bad.toml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported scheme")
}

func TestReadConf_AllBadChecksFail(t *testing.T) {
	_, err := readTestConfig("upd_test_allbad.toml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported scheme")
}

func TestGetChecks_NoChecks(t *testing.T) {
	var conf Configuration

	_, err := conf.GetChecks()
	assert.ErrorIs(t, err, ErrNoChecks)
}

func (suite *TestSuite) TestGetChecks() {
	checklist, err := suite.conf.GetChecks()
	suite.Require().NoError(err)

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
	suite.Equal("https", httpProbe.Scheme())
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
	suite.NotNil(conf.Port)
	suite.Equal(8080, conf.Port)
}

func TestReadConf_envsubst(t *testing.T) {
	t.Setenv("UPD_TEST_TIMEOUT", "3s")

	conf, err := readTestConfig("upd_test_envvar.toml")
	require.NoError(t, err)
	assert.Equal(t, Duration(3*time.Second), conf.Checks.TimeOut)
}

func TestReadConf_envsubst_missing(t *testing.T) {
	_, err := readTestConfig("upd_test_envvar.toml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment variable")
	assert.Contains(t, err.Error(), "not set")
}

func TestDNSCheckValidation_MissingDomain(t *testing.T) {
	var conf Configuration

	conf.Checks.List.Ordered = []string{"dns://8.8.4.4/"}

	_, err := conf.GetChecks()
	require.Error(t, err)
	assert.ErrorIs(t, err, check.ErrDNSMissingDomain)
}

func TestDNSCheckValidation_MissingResolver(t *testing.T) {
	conf, err := readTestConfig("upd_test_dns_missing_resolver.toml")
	require.NoError(t, err)

	_, checkErr := conf.GetChecks()
	require.Error(t, checkErr)
	assert.ErrorIs(t, checkErr, check.ErrDNSMissingResolver)
}

func TestLogSetup(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"empty defaults to info", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			conf := &Configuration{LogLevel: tt.logLevel}
			conf.logSetup()
		})
	}
}

func TestExpandEnvVars_set(t *testing.T) {
	t.Setenv("UPD_EXPAND_TEST", "expanded_value")

	content := []byte(`timeout = "prefix_${UPD_EXPAND_TEST}_suffix"`)
	result, err := expandEnvVars(content)
	require.NoError(t, err)
	assert.Equal(t, []byte(`timeout = "prefix_expanded_value_suffix"`), result)
}

func TestExpandEnvVars_unset(t *testing.T) {
	content := []byte(`timeout = "${NONEXISTENT_VAR}"`)
	_, err := expandEnvVars(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NONEXISTENT_VAR")
}

func TestExpandEnvVars_noVar(t *testing.T) {
	content := []byte(`timeout = "2000ms"`)
	result, err := expandEnvVars(content)
	require.NoError(t, err)
	assert.Equal(t, []byte(`timeout = "2000ms"`), result)
}

func TestExpandEnvVars_unbracedNotExpanded(t *testing.T) {
	content := []byte(`exec = "$PATH"`)
	result, err := expandEnvVars(content)
	require.NoError(t, err)
	assert.Equal(t, []byte(`exec = "$PATH"`), result)
}

func TestExpandEnvVars_multiple(t *testing.T) {
	t.Setenv("UPD_A", "a_val")
	t.Setenv("UPD_B", "b_val")

	content := []byte("x = \"${UPD_A}\"\ny = \"${UPD_B}\"")
	result, err := expandEnvVars(content)
	require.NoError(t, err)
	assert.Equal(t, []byte("x = \"a_val\"\ny = \"b_val\""), result)
}

func TestExpandEnvVars_nilContent(t *testing.T) {
	result, err := expandEnvVars(nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}
