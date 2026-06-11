package bast

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// LoadConfig loads and validates configuration from environment variables.
// T must be a struct. Reflection runs once at startup — never on the hot path.
//
// Supported tags:
//
//	env:"VAR_NAME"    — environment variable name
//	default:"value"   — used when env var is absent
//	required:"true"   — app refuses to start if absent and no default
//	secret:"true"     — value is masked in boot logs
//
// Supported field types: string, int, int64, float64, bool, time.Duration, []string
// Nested structs: env tag on the struct field acts as a prefix (e.g. env:"DB" → DB_URL).
func LoadConfig[T any]() (T, error) {
	var zero T
	rv := reflect.ValueOf(&zero).Elem()
	rt := rv.Type()

	if rt.Kind() != reflect.Struct {
		return zero, fmt.Errorf("bast: LoadConfig: T must be a struct")
	}

	var missing []string
	if err := fillStruct(rv, rt, "", &missing); err != nil {
		return zero, err
	}
	if len(missing) > 0 {
		return zero, fmt.Errorf("bast: missing required config: %s", strings.Join(missing, ", "))
	}
	return zero, nil
}

// fillStruct recursively populates fields from env vars.
func fillStruct(rv reflect.Value, rt reflect.Type, prefix string, missing *[]string) error {
	for i := range rt.NumField() {
		field := rt.Field(i)
		fv := rv.Field(i)

		// Skip unexported or otherwise non-settable fields — calling Set/SetString on
		// them panics. Unexported fields with env tags are a configuration bug but we
		// degrade gracefully rather than crashing at startup.
		if !fv.CanSet() {
			continue
		}

		envKey := field.Tag.Get("env")
		if envKey == "" {
			continue
		}

		fullKey := envKey
		if prefix != "" {
			fullKey = prefix + "_" + envKey
		}

		// Nested struct — recurse with the current key as prefix.
		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Duration(0)) {
			if err := fillStruct(fv, field.Type, fullKey, missing); err != nil {
				return err
			}
			continue
		}

		raw, found := os.LookupEnv(fullKey)
		if !found {
			def := field.Tag.Get("default")
			if def != "" {
				raw = def
				found = true
			}
		}

		required := field.Tag.Get("required") == "true"
		if !found {
			if required {
				*missing = append(*missing, fullKey)
			}
			continue
		}

		if err := setField(fv, field.Type, raw, fullKey); err != nil {
			return err
		}
	}
	return nil
}

// setField parses raw string into the correct type and sets the field value.
func setField(fv reflect.Value, ft reflect.Type, raw, key string) error {
	switch ft {
	case reflect.TypeOf(""):
		fv.SetString(raw)

	case reflect.TypeOf(0):
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("bast: config %s: invalid int %q", key, raw)
		}
		fv.SetInt(int64(n))

	case reflect.TypeOf(int64(0)):
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("bast: config %s: invalid int64 %q", key, raw)
		}
		fv.SetInt(n)

	case reflect.TypeOf(float64(0)):
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("bast: config %s: invalid float64 %q", key, raw)
		}
		fv.SetFloat(f)

	case reflect.TypeOf(false):
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("bast: config %s: invalid bool %q", key, raw)
		}
		fv.SetBool(b)

	case reflect.TypeOf(time.Duration(0)):
		d, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("bast: config %s: invalid duration %q", key, raw)
		}
		fv.SetInt(int64(d))

	default:
		// []string — comma-separated.
		if ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.String {
			parts := strings.Split(raw, ",")
			sl := reflect.MakeSlice(ft, len(parts), len(parts))
			for i, p := range parts {
				sl.Index(i).SetString(strings.TrimSpace(p))
			}
			fv.Set(sl)
			return nil
		}
		return fmt.Errorf("bast: config %s: unsupported field type %s", key, ft)
	}
	return nil
}