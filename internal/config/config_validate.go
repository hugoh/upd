package config

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/hugoh/upd/internal/check"
)

var (
	errDurationMustBePositive = errors.New("must be greater than 0")
	errInvalidURI             = errors.New("must be a valid URI")
	errMustNotBeNegative      = errors.New("must not be negative")
	errPortOutOfRange         = errors.New("must be between 1 and 65535")
	errInvalidLogLevel        = errors.New("must be one of: debug, info, warn")
	errUnsupportedScheme      = errors.New("unsupported scheme")
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

	if !reflect.ValueOf(c.DownAction).IsZero() {
		errs = appendErr(errs, "downAction", c.validateDownAction())
	}

	if !reflect.ValueOf(c.Stats).IsZero() {
		errs = appendErr(errs, "stats", c.validateStats())
	}

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
	errs = appendErr(errs, "readTimeout", checkNonNegative(c.Stats.ReadTimeout.StdDuration()))
	errs = appendErr(errs, "writeTimeout", checkNonNegative(c.Stats.WriteTimeout.StdDuration()))
	errs = appendErr(errs, "idleTimeout", checkNonNegative(c.Stats.IdleTimeout.StdDuration()))

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

	for i, s := range uris {
		// Same parser as GetChecksCat so validation matches what gets built.
		parsed, err := url.Parse(s)
		if err != nil {
			errs = append(errs, fmt.Errorf("[%d]: %w", i, errInvalidURI))

			continue
		}

		switch parsed.Scheme {
		case check.DNS, check.HTTP, check.HTTPS, check.TCP:
		default:
			errs = append(errs, fmt.Errorf("[%d]: %w: %q", i, errUnsupportedScheme, parsed.Scheme))
		}
	}

	return errors.Join(errs...)
}
