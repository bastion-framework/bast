package bast_test

import (
	"testing"
	"time"

	"github.com/bastion-framework/bast"
)

func TestLoadConfig_BasicFields(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("APP_NAME", "bast-test")

	type Cfg struct {
		Port    int    `env:"PORT"     default:"8080"`
		AppName string `env:"APP_NAME" required:"true"`
	}

	cfg, err := bast.LoadConfig[Cfg]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.AppName != "bast-test" {
		t.Errorf("AppName = %q, want bast-test", cfg.AppName)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	type Cfg struct {
		Port int `env:"BAST_TEST_PORT_DEFAULT" default:"3000"`
	}

	cfg, err := bast.LoadConfig[Cfg]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want 3000 (default)", cfg.Port)
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	type Cfg struct {
		Secret string `env:"BAST_TEST_MISSING_SECRET" required:"true"`
	}

	_, err := bast.LoadConfig[Cfg]()
	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
}

func TestLoadConfig_BoolField(t *testing.T) {
	t.Setenv("BAST_TEST_DEBUG", "true")

	type Cfg struct {
		Debug bool `env:"BAST_TEST_DEBUG" default:"false"`
	}

	cfg, err := bast.LoadConfig[Cfg]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
}

func TestLoadConfig_DurationField(t *testing.T) {
	t.Setenv("BAST_TEST_TIMEOUT", "30s")

	type Cfg struct {
		Timeout time.Duration `env:"BAST_TEST_TIMEOUT"`
	}

	cfg, err := bast.LoadConfig[Cfg]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestLoadConfig_StringSlice(t *testing.T) {
	t.Setenv("BAST_TEST_ORIGINS", "https://a.com,https://b.com,https://c.com")

	type Cfg struct {
		Origins []string `env:"BAST_TEST_ORIGINS"`
	}

	cfg, err := bast.LoadConfig[Cfg]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Origins) != 3 {
		t.Errorf("Origins len = %d, want 3", len(cfg.Origins))
	}
	if cfg.Origins[1] != "https://b.com" {
		t.Errorf("Origins[1] = %q, want https://b.com", cfg.Origins[1])
	}
}

func TestLoadConfig_NestedStruct(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/bast")
	t.Setenv("DB_MAX_CONNS", "20")

	type DBConfig struct {
		URL      string `env:"URL"       required:"true"`
		MaxConns int    `env:"MAX_CONNS" default:"10"`
	}
	type Cfg struct {
		Database DBConfig `env:"DB"`
	}

	cfg, err := bast.LoadConfig[Cfg]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database.URL != "postgres://localhost/bast" {
		t.Errorf("Database.URL = %q, want postgres://localhost/bast", cfg.Database.URL)
	}
	if cfg.Database.MaxConns != 20 {
		t.Errorf("Database.MaxConns = %d, want 20", cfg.Database.MaxConns)
	}
}

func TestLoadConfig_MissingRequiredReportsAll(t *testing.T) {
	type Cfg struct {
		A string `env:"BAST_TEST_MISSING_A" required:"true"`
		B string `env:"BAST_TEST_MISSING_B" required:"true"`
	}

	_, err := bast.LoadConfig[Cfg]()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !containsStr(msg, "BAST_TEST_MISSING_A") || !containsStr(msg, "BAST_TEST_MISSING_B") {
		t.Errorf("error should name all missing vars, got: %s", msg)
	}
}

func TestLoadConfig_UnexportedFields_DoNotPanic(t *testing.T) {
	t.Setenv("BAST_TEST_UNEXP_SECRET", "must-not-panic")
	type cfg struct {
		Port   int    `env:"BAST_TEST_UNEXP_PORT" default:"4321"`
		secret string `env:"BAST_TEST_UNEXP_SECRET"` // unexported — must be skipped, not panic
	}
	got, err := bast.LoadConfig[cfg]()
	if err != nil {
		t.Fatalf("unexpected error for struct with unexported field: %v", err)
	}
	if got.Port != 4321 {
		t.Errorf("Port = %d, want 4321", got.Port)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := range s {
				if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}