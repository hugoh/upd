package config

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/hugoh/upd/internal/status"
)

var (
	errDurationMustBePositive = errors.New("must be greater than 0")
	errInvalidURI             = errors.New("must be a valid URI")
	errMustNotBeNegative      = errors.New("must not be negative")
	errPortOutOfRange         = errors.New("must be between 1 and 65535")
	errInvalidLogLevel        = errors.New("must be one of: debug, info, warn")
	errUnsupportedScheme      = errors.New("unsupported scheme")
	errTooManyBuckets         = errors.New("report period needs too many buckets")
)

func appendErr(errs []error, key string, err error) []error {
	if err != nil {
		return append(errs, fmt.Errorf("%s: %w", key, err))
	}

	return errs
}

// Validate checks that the configuration has all required fields with valid values.
func (c Configuration) Validate() error {
	var errs []error

	errs = appendErr(errs, "checks", c.validateChecks())
	errs = appendErr(errs, "downAction", c.validateDownAction())
	errs = appendErr(errs, "stats", c.validateStats())

	if c.LogLevel != "" {
		errs = appendErr(errs, "logLevel", validateLogLevel(c.LogLevel))
	}

	return errors.Join(errs...)
}

func (c Configuration) validateChecks() error {
	var errs []error

	errs = appendErr(errs, "every.normal", validatePositiveDuration(c.Checks.Every.Normal))
	errs = appendErr(errs, "every.down", validatePositiveDuration(c.Checks.Every.Down))
	errs = appendErr(errs, "timeout", validatePositiveDuration(c.Checks.TimeOut))
	errs = appendErr(errs, "list.ordered", validateURIs(c.Checks.List.Ordered))
	errs = appendErr(errs, "list.shuffled", validateURIs(c.Checks.List.Shuffled))

	return errors.Join(errs...)
}

func (c Configuration) validateDownAction() error {
	var errs []error

	errs = appendErr(errs, "every.after", checkNonNegative(time.Duration(c.DownAction.Every.After)))
	errs = appendErr(
		errs,
		"every.repeat",
		checkNonNegative(time.Duration(c.DownAction.Every.Repeat)),
	)
	errs = appendErr(
		errs,
		"every.expBackoffLimit",
		checkNonNegative(time.Duration(c.DownAction.Every.BackoffLimit)),
	)

	return errors.Join(errs...)
}

func (c Configuration) validateStats() error {
	var errs []error

	errs = appendErr(errs, "port", validatePort(c.Stats.Port))
	errs = appendErr(errs, "buckets.min", checkNonNegativeInt(c.Stats.Buckets.Min))
	errs = appendErr(
		errs,
		"buckets.maxSpan",
		checkNonNegative(c.Stats.Buckets.MaxSpan.StdDuration()),
	)
	errs = appendErr(errs, "reports", c.Stats.validateReports())
	errs = appendErr(errs, "readTimeout", checkNonNegative(c.Stats.ReadTimeout.StdDuration()))
	errs = appendErr(errs, "writeTimeout", checkNonNegative(c.Stats.WriteTimeout.StdDuration()))
	errs = appendErr(errs, "idleTimeout", checkNonNegative(c.Stats.IdleTimeout.StdDuration()))

	return errors.Join(errs...)
}

func (s StatsConfig) validateReports() error {
	bucketCfg := s.GetBucketConfig()

	var errs []error

	for idx, report := range s.Reports {
		period := report.StdDuration()
		if period <= 0 {
			errs = append(errs, fmt.Errorf("[%d]: %w", idx, errDurationMustBePositive))

			continue
		}

		if n := bucketCfg.BucketCount(period); n > status.MaxBucketsPerPeriod {
			errs = append(errs, fmt.Errorf(
				"[%d]: %w (%d > %d): increase buckets.maxSpan",
				idx, errTooManyBuckets, n, status.MaxBucketsPerPeriod))
		}
	}

	return errors.Join(errs...)
}

func validatePositiveDuration(d Duration) error {
	if time.Duration(d) <= 0 {
		return errDurationMustBePositive
	}

	return nil
}

func checkNonNegative(d time.Duration) error {
	if d < 0 {
		return errMustNotBeNegative
	}

	return nil
}

func checkNonNegativeInt(n int) error {
	if n < 0 {
		return errMustNotBeNegative
	}

	return nil
}

func validatePort(port int) error {
	if port < 0 || port > 65535 {
		return errPortOutOfRange
	}

	return nil
}

func validateLogLevel(level string) error {
	switch level {
	case "debug", "info", "warn":
		return nil
	default:
		return errInvalidLogLevel
	}
}

func validateURIs(uris []string) error {
	var errs []error

	for idx, uri := range uris {
		parsed, err := url.Parse(uri)
		if err != nil {
			errs = append(errs, fmt.Errorf("[%d]: %w", idx, errInvalidURI))

			continue
		}

		// Validate by attempting the same construction GetChecksCat performs,
		// so a config that passes validation is guaranteed to build.
		if _, err := probeFromURL(parsed); err != nil {
			errs = append(errs, fmt.Errorf("[%d]: %w", idx, err))
		}
	}

	return errors.Join(errs...)
}
