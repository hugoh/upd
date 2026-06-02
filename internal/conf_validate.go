package internal

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/hugoh/upd/internal/types"
)

var (
	errDurationMustBePositive = errors.New("must be greater than 0")
	errInvalidURI             = errors.New("must be a valid URI")
	errNotDuration            = errors.New("value is not a types.Duration")
	errNotString              = errors.New("value is not a string")
)

// Validate checks that the configuration has all required fields with valid values.
func (c Configuration) Validate() error {
	if err := validation.ValidateStruct(&c,
		validation.Field(&c.Checks, validation.Required,
			validation.By(c.validateChecks),
		),
		validation.Field(&c.DownAction,
			validation.When(!reflect.ValueOf(c.DownAction).IsZero(),
				validation.By(c.validateDownAction),
			),
		),
		validation.Field(&c.Stats,
			validation.When(!reflect.ValueOf(c.Stats).IsZero(),
				validation.By(c.validateStats),
			),
		),
		validation.Field(&c.LogLevel,
			validation.When(c.LogLevel != "",
				validation.In("trace", "debug", "info", "warn"),
			),
		),
	); err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	return nil
}

func (c Configuration) validateChecks(_ any) error {
	var errs []error

	if err := c.validateEvery(nil); err != nil {
		errs = append(errs, fmt.Errorf("every: %w", err))
	}

	if err := c.validateList(nil); err != nil {
		errs = append(errs, fmt.Errorf("list: %w", err))
	}

	if err := validation.Validate(c.Checks.TimeOut, validation.Required,
		validation.By(validatePositiveDuration),
	); err != nil {
		errs = append(errs, fmt.Errorf("timeout: %w", err))
	}

	return errors.Join(errs...)
}

func (c Configuration) validateEvery(_ any) error {
	var errs []error

	if err := validation.Validate(c.Checks.Every.Normal, validation.Required,
		validation.By(validatePositiveDuration),
	); err != nil {
		errs = append(errs, fmt.Errorf("normal: %w", err))
	}

	if err := validation.Validate(c.Checks.Every.Down, validation.Required,
		validation.By(validatePositiveDuration),
	); err != nil {
		errs = append(errs, fmt.Errorf("down: %w", err))
	}

	return errors.Join(errs...)
}

func (c Configuration) validateList(_ any) error {
	var errs []error

	if len(c.Checks.List.Ordered) > 0 {
		if err := validation.Validate(c.Checks.List.Ordered,
			validation.Each(validation.By(validateURI)),
		); err != nil {
			errs = append(errs, fmt.Errorf("ordered: %w", err))
		}
	}

	if len(c.Checks.List.Shuffled) > 0 {
		if err := validation.Validate(c.Checks.List.Shuffled,
			validation.Each(validation.By(validateURI)),
		); err != nil {
			errs = append(errs, fmt.Errorf("shuffled: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (c Configuration) validateDownAction(_ any) error {
	var errs []error

	if c.DownAction.Every.After != 0 {
		if err := validation.Validate(c.DownAction.Every.After,
			validation.By(validatePositiveDuration),
		); err != nil {
			errs = append(errs, fmt.Errorf("after: %w", err))
		}
	}

	if c.DownAction.Every.Repeat != 0 {
		if err := validation.Validate(c.DownAction.Every.Repeat,
			validation.By(validatePositiveDuration),
		); err != nil {
			errs = append(errs, fmt.Errorf("repeat: %w", err))
		}
	}

	if c.DownAction.Every.BackoffLimit != 0 {
		if err := validation.Validate(c.DownAction.Every.BackoffLimit,
			validation.Min(time.Duration(0)),
		); err != nil {
			errs = append(errs, fmt.Errorf("expBackoffLimit: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (c Configuration) validateStats(_ any) error {
	var errs []error

	if c.Stats.Port != 0 {
		if err := validation.Validate(c.Stats.Port,
			validation.Min(1),
			validation.Max(65535), //nolint:mnd // well-known max port
		); err != nil {
			errs = append(errs, fmt.Errorf("port: %w", err))
		}
	}

	if c.Stats.ReadTimeout != 0 {
		if err := validation.Validate(c.Stats.ReadTimeout,
			validation.Min(time.Duration(0)),
		); err != nil {
			errs = append(errs, fmt.Errorf("readTimeout: %w", err))
		}
	}

	if c.Stats.WriteTimeout != 0 {
		if err := validation.Validate(c.Stats.WriteTimeout,
			validation.Min(time.Duration(0)),
		); err != nil {
			errs = append(errs, fmt.Errorf("writeTimeout: %w", err))
		}
	}

	if c.Stats.IdleTimeout != 0 {
		if err := validation.Validate(c.Stats.IdleTimeout,
			validation.Min(time.Duration(0)),
		); err != nil {
			errs = append(errs, fmt.Errorf("idleTimeout: %w", err))
		}
	}

	return errors.Join(errs...)
}

func validatePositiveDuration(value any) error {
	d, ok := value.(types.Duration)
	if !ok {
		return errNotDuration
	}

	if time.Duration(d) <= 0 {
		return errDurationMustBePositive
	}

	return nil
}

func validateURI(value any) error {
	s, ok := value.(string)
	if !ok {
		return errNotString
	}

	_, err := url.ParseRequestURI(s)
	if err != nil {
		return errInvalidURI
	}

	return nil
}
