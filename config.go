package serverconfig

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Verifier interface {
	Verify() error
}

var (
	durationType = reflect.TypeOf(time.Duration(0))
)

type Config struct {
	Logging  LoggingConfig `yaml:"logging"`
	Database MySQLDatabase `yaml:"database"`
	Redis    RedisConfig   `yaml:"redis"`
	SMTP     SMTPConfig    `yaml:"smtp"`
	HTTP     HTTPConfig    `yaml:"http"`
}

// Read reads a YAML file into a configuration struct.  Anything tagges with 'ENV' can have an overriding value
// in the OS environment which, if existing, will override any values read from the YAML file.
// Any sub-structs satisfying the Verifier interface will get that called to verify the data read.
func Read(filename string, cfg any) error {
	var (
		b   []byte
		err error
	)

	err = validateConfigPointer(cfg)
	if err != nil {
		return err
	}

	b, err = os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("unable to read configuration file: %s, error: %w", filename, err)
	}

	err = yaml.Unmarshal(b, cfg)
	if err != nil {
		return fmt.Errorf("unable to parse configuration file: %s, error: %w", filename, err)
	}

	err = applyEnvOverrides(cfg)
	if err != nil {
		return err
	}

	err = verifySubStructs(cfg)
	if err != nil {
		return err
	}

	return nil
}

func validateConfigPointer(cfg any) error {
	var value reflect.Value

	value = reflect.ValueOf(cfg)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("config must be a non-nil pointer to a struct")
	}
	if value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("cfg must point to a struct")
	}

	return nil
}

func applyEnvOverrides(cfg any) error {
	var (
		value reflect.Value
		err   error
	)

	value = reflect.ValueOf(cfg)
	err = applyEnvOverridesValue(value, "")
	if err != nil {
		return err
	}

	return nil
}

func applyEnvOverridesValue(value reflect.Value, path string) error {
	var (
		err       error
		i         int
		field     reflect.Value
		fieldDef  reflect.StructField
		fieldPath string
		envName   string
		envValue  string
		found     bool
	)

	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil
	}

	for i = 0; i < value.NumField(); i++ {
		field = value.Field(i)
		fieldDef = value.Type().Field(i)
		if len(fieldDef.PkgPath) > 0 {
			continue
		}

		if len(path) == 0 {
			fieldPath = fieldDef.Name
		} else {
			fieldPath = path + "." + fieldDef.Name
		}

		envName = fieldDef.Tag.Get("env")
		if len(envName) > 0 {
			envValue, found = os.LookupEnv(envName)
			if found {
				err = setValueFromEnv(field, envValue)
				if err != nil {
					return fmt.Errorf("invalid value for env %s (%s): %w", envName, fieldPath, err)
				}
			}
		}

		err = applyEnvOverridesValue(field, fieldPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func setValueFromEnv(field reflect.Value, raw string) error {
	var (
		err         error
		parsedBool  bool
		parsedInt   int64
		parsedUint  uint64
		parsedFloat float64
		duration    time.Duration
		parts       []string
		slice       reflect.Value
		i           int
	)

	if !field.CanSet() {
		return fmt.Errorf("field is not settable")
	}

	if field.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return setValueFromEnv(field.Elem(), raw)
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
		return nil
	case reflect.Bool:
		parsedBool, err = strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("expected bool, got %q", raw)
		}
		field.SetBool(parsedBool)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == durationType {
			duration, err = time.ParseDuration(raw)
			if err != nil {
				return fmt.Errorf("expected duration, got %q", raw)
			}
			field.SetInt(int64(duration))
			return nil
		}
		parsedInt, err = strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("expected integer, got %q", raw)
		}
		field.SetInt(parsedInt)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsedUint, err = strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("expected unsigned integer, got %q", raw)
		}
		field.SetUint(parsedUint)
		return nil
	case reflect.Float32, reflect.Float64:
		parsedFloat, err = strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("expected float, got %q", raw)
		}
		field.SetFloat(parsedFloat)
		return nil
	case reflect.Slice:
		if field.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported slice type %s", field.Type())
		}
		if len(strings.TrimSpace(raw)) == 0 {
			field.Set(reflect.MakeSlice(field.Type(), 0, 0))
			return nil
		}
		parts = strings.Split(raw, ",")
		slice = reflect.MakeSlice(field.Type(), len(parts), len(parts))
		for i = 0; i < len(parts); i++ {
			slice.Index(i).SetString(strings.TrimSpace(parts[i]))
		}
		field.Set(slice)
		return nil
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
}

func verifySubStructs(cfg any) error {
	var (
		value reflect.Value
		err   error
	)

	value = reflect.ValueOf(cfg)
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	if !value.IsValid() || value.Kind() != reflect.Struct {
		return fmt.Errorf("config must point to a struct")
	}

	err = verifyStructValues(value, value.Type().Name())
	if err != nil {
		return err
	}

	return nil
}

func verifyStructValues(value reflect.Value, path string) error {
	var (
		err       error
		i         int
		field     reflect.Value
		fieldDef  reflect.StructField
		fieldPath string
	)

	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil
	}

	for i = 0; i < value.NumField(); i++ {
		field = value.Field(i)
		fieldDef = value.Type().Field(i)
		if len(fieldDef.PkgPath) > 0 {
			continue
		}
		if len(path) == 0 {
			fieldPath = fieldDef.Name
		} else {
			fieldPath = path + "." + fieldDef.Name
		}

		err = callVerify(field, fieldPath)
		if err != nil {
			return err
		}

		err = verifyStructValues(field, fieldPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func callVerify(value reflect.Value, path string) error {
	var (
		err      error
		verifier Verifier
		ok       bool
	)

	if !value.IsValid() {
		return nil
	}

	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		if value.CanInterface() {
			verifier, ok = value.Interface().(Verifier)
			if ok {
				err = verifier.Verify()
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				return nil
			}
		}
		value = value.Elem()
	}

	if value.CanAddr() && value.Addr().CanInterface() {
		verifier, ok = value.Addr().Interface().(Verifier)
		if ok {
			err = verifier.Verify()
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			return nil
		}
	}

	if value.CanInterface() {
		verifier, ok = value.Interface().(Verifier)
		if ok {
			err = verifier.Verify()
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
	}

	return nil
}
