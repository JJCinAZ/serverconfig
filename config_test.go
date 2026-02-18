package serverconfig

import (
	"errors"
	"fmt"
	"log/syslog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var errVerifyBoom = errors.New("verify boom")

type readVerifySection struct {
	Name     string `yaml:"name"`
	Verified bool   `yaml:"-"`
}

func (s *readVerifySection) Verify() error {
	s.Verified = true
	if len(s.Name) == 0 {
		return fmt.Errorf("missing name")
	}
	return nil
}

type readRuntimeSection struct {
	Enabled    bool          `yaml:"enabled" env:"APP_ENABLED"`
	Port       int           `yaml:"port" env:"APP_PORT"`
	Hosts      []string      `yaml:"hosts" env:"APP_HOSTS"`
	Timeout    time.Duration `yaml:"timeout" env:"APP_TIMEOUT"`
	MaxRetries *int          `yaml:"max_retries" env:"APP_MAX_RETRIES"`
}

type readEnvConfig struct {
	Section readVerifySection  `yaml:"section"`
	Runtime readRuntimeSection `yaml:"runtime"`
}

type readFailSection struct {
	Token string `yaml:"token"`
}

func (s *readFailSection) Verify() error {
	return errVerifyBoom
}

type readFailConfig struct {
	Check readFailSection `yaml:"check"`
}

func TestReadRejectsInvalidTargets(t *testing.T) {
	var (
		nilConfigPtr *Config
		testCases    []struct {
			name       string
			target     any
			wantSubstr string
		}
		i  int
		tc struct {
			name       string
			target     any
			wantSubstr string
		}
		err error
	)

	testCases = []struct {
		name       string
		target     any
		wantSubstr string
	}{
		{name: "nil-interface", target: nil, wantSubstr: "non-nil pointer"},
		{name: "struct-value", target: Config{}, wantSubstr: "non-nil pointer"},
		{name: "pointer-to-non-struct", target: new(int), wantSubstr: "point to a struct"},
		{name: "nil-struct-pointer", target: nilConfigPtr, wantSubstr: "non-nil pointer"},
	}

	for i = 0; i < len(testCases); i++ {
		tc = testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			err = Read("ignored.yaml", tc.target)
			if errors.Is(err, nil) {
				t.Fatalf("expected error for target %T", tc.target)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantSubstr, err.Error())
			}
		})
	}
}

func TestReadAppliesEnvOverridesAndCallsVerify(t *testing.T) {
	var (
		yamlBody string
		path     string
		cfg      readEnvConfig
		err      error
	)

	yamlBody = "section:\n  name: from-yaml\nruntime:\n  enabled: false\n  port: 8080\n  hosts:\n    - host-a\n  timeout: 5s\n  max_retries: 1\n"
	path = writeTempConfig(t, yamlBody)

	t.Setenv("APP_ENABLED", "true")
	t.Setenv("APP_PORT", "9191")
	t.Setenv("APP_HOSTS", "host-x, host-y")
	t.Setenv("APP_TIMEOUT", "45s")
	t.Setenv("APP_MAX_RETRIES", "7")

	err = Read(path, &cfg)
	if !errors.Is(err, nil) {
		t.Fatalf("Read returned error: %v", err)
	}

	if !cfg.Section.Verified {
		t.Fatalf("expected section verifier to be called")
	}
	if cfg.Runtime.Enabled != true {
		t.Fatalf("expected runtime.enabled override, got %v", cfg.Runtime.Enabled)
	}
	if cfg.Runtime.Port != 9191 {
		t.Fatalf("expected runtime.port override, got %d", cfg.Runtime.Port)
	}
	if len(cfg.Runtime.Hosts) != 2 || cfg.Runtime.Hosts[0] != "host-x" || cfg.Runtime.Hosts[1] != "host-y" {
		t.Fatalf("unexpected runtime.hosts: %#v", cfg.Runtime.Hosts)
	}
	if cfg.Runtime.Timeout != 45*time.Second {
		t.Fatalf("expected runtime.timeout override, got %s", cfg.Runtime.Timeout)
	}
	if cfg.Runtime.MaxRetries == nil || *cfg.Runtime.MaxRetries != 7 {
		t.Fatalf("expected runtime.max_retries override, got %#v", cfg.Runtime.MaxRetries)
	}
}

func TestReadReturnsWrappedVerifyError(t *testing.T) {
	var (
		yamlBody string
		path     string
		cfg      readFailConfig
		err      error
	)

	yamlBody = "check:\n  token: abc\n"
	path = writeTempConfig(t, yamlBody)

	err = Read(path, &cfg)
	if errors.Is(err, nil) {
		t.Fatalf("expected verify error")
	}
	if !errors.Is(err, errVerifyBoom) {
		t.Fatalf("expected wrapped verify error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Check") {
		t.Fatalf("expected field path in error, got: %v", err)
	}
}

func TestReadWithDefaultConfig(t *testing.T) {
	var (
		yamlBody  string
		path      string
		cfg       Config
		err       error
		expectedP syslog.Priority
	)

	yamlBody = "logging:\n  syslog_enabled: true\ndatabase:\n  server: db.local:3306\n  user: app\n  password: from-yaml\n  db: maindb\nredis:\n  server: redis.local:6379\nsmtp:\n  server: smtp.local\n  port: 587\n  from: noreply@example.com\nhttp:\n  bindaddr: :80\n  sslbindaddr: :443\n  templatepath: ./templates\n  externalhostname:\n    - example.com\n  skiphostnametest: true\n  static_cert:\n    certfile: /tmp/cert.pem\n    privatekeyfile: /tmp/key.pem\n"
	path = writeTempConfig(t, yamlBody)

	t.Setenv("DBPASS", "from-env")

	err = Read(path, &cfg)
	if !errors.Is(err, nil) {
		t.Fatalf("Read returned error: %v", err)
	}

	if cfg.Database.Password != "from-env" {
		t.Fatalf("expected env-overridden DB password, got %q", cfg.Database.Password)
	}
	if cfg.Database.ConnectString != "app:from-env@tcp(db.local:3306)/maindb" {
		t.Fatalf("unexpected connect string: %q", cfg.Database.ConnectString)
	}

	expectedP = syslog.Priority(int(syslog.LOG_LOCAL5) | int(syslog.LOG_INFO))
	if cfg.Logging.Syslog.Priority() != expectedP {
		t.Fatalf("unexpected default logging priority: %d", cfg.Logging.Syslog.Priority())
	}
}

func TestMySQLDatabaseVerify(t *testing.T) {
	var (
		cfg MySQLDatabase
		err error
	)

	cfg = MySQLDatabase{
		Server:   "db.example.com:3306",
		User:     "dbuser",
		Password: "dbpass",
		DB:       "appdb",
	}

	err = cfg.Verify()
	if !errors.Is(err, nil) {
		t.Fatalf("Verify returned error: %v", err)
	}
	if cfg.ConnectString != "dbuser:dbpass@tcp(db.example.com:3306)/appdb" {
		t.Fatalf("unexpected connect string: %q", cfg.ConnectString)
	}
}

func TestMySQLDatabaseVerifyMissingRequired(t *testing.T) {
	var (
		testCases []struct {
			name       string
			cfg        MySQLDatabase
			wantSubstr string
		}
		i  int
		tc struct {
			name       string
			cfg        MySQLDatabase
			wantSubstr string
		}
		err error
	)

	testCases = []struct {
		name       string
		cfg        MySQLDatabase
		wantSubstr string
	}{
		{name: "missing-password", cfg: MySQLDatabase{Server: "db:3306", User: "u", DB: "x"}, wantSubstr: "missing Database Password"},
		{name: "missing-user", cfg: MySQLDatabase{Server: "db:3306", Password: "p", DB: "x"}, wantSubstr: "missing Database User"},
		{name: "missing-server", cfg: MySQLDatabase{User: "u", Password: "p", DB: "x"}, wantSubstr: "missing Database Server"},
		{name: "missing-port", cfg: MySQLDatabase{Server: "db", User: "u", Password: "p", DB: "x"}, wantSubstr: "should specify a port"},
	}

	for i = 0; i < len(testCases); i++ {
		tc = testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			err = tc.cfg.Verify()
			if errors.Is(err, nil) {
				t.Fatalf("expected error for case %q", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantSubstr, err.Error())
			}
		})
	}
}

func TestPostgresDatabaseVerify(t *testing.T) {
	var (
		cfg PostgresDatabase
		err error
	)

	cfg = PostgresDatabase{
		Server:   "db.example.com:5432",
		User:     "dbuser",
		Password: "dbpass",
		DB:       "appdb",
	}

	err = cfg.Verify()
	if !errors.Is(err, nil) {
		t.Fatalf("Verify returned error: %v", err)
	}
	if cfg.ConnectString != "postgres://dbuser:dbpass@db.example.com:5432/appdb" {
		t.Fatalf("unexpected connect string: %q", cfg.ConnectString)
	}
}

func TestRedisConfigVerifyDefaults(t *testing.T) {
	var (
		cfg RedisConfig
		err error
	)

	cfg = RedisConfig{Server: "redis.example.com:6379"}

	err = cfg.Verify()
	if !errors.Is(err, nil) {
		t.Fatalf("Verify returned error: %v", err)
	}

	if cfg.MaxIdle != 3 {
		t.Fatalf("expected default MaxIdle=3, got %d", cfg.MaxIdle)
	}
	if cfg.MaxActive != 32 {
		t.Fatalf("expected default MaxActive=32, got %d", cfg.MaxActive)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Fatalf("expected default IdleTimeout=60s, got %s", cfg.IdleTimeout)
	}
}

func TestLoggingConfigVerify(t *testing.T) {
	var (
		cfg       LoggingConfig
		err       error
		expectedP syslog.Priority
	)

	err = cfg.Verify()
	if !errors.Is(err, nil) {
		t.Fatalf("Verify returned error: %v", err)
	}

	if cfg.Syslog.FacilityString != "LOG_LOCAL5" {
		t.Fatalf("unexpected default facility: %q", cfg.Syslog.FacilityString)
	}
	if cfg.Syslog.SeverityString != "LOG_INFO" {
		t.Fatalf("unexpected default severity: %q", cfg.Syslog.SeverityString)
	}

	expectedP = syslog.Priority(int(syslog.LOG_LOCAL5) | int(syslog.LOG_INFO))
	if cfg.Syslog.Priority() != expectedP {
		t.Fatalf("unexpected priority: %d", cfg.Syslog.Priority())
	}

	cfg.Syslog.FacilityString = "INVALID"
	err = cfg.Verify()
	if errors.Is(err, nil) {
		t.Fatalf("expected invalid facility error")
	}
	if !strings.Contains(err.Error(), "invalid logging facility") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPConfigVerify(t *testing.T) {
	var (
		testCases []struct {
			name       string
			cfg        HTTPConfig
			wantSubstr string
		}
		i  int
		tc struct {
			name       string
			cfg        HTTPConfig
			wantSubstr string
		}
		err error
	)

	testCases = []struct {
		name       string
		cfg        HTTPConfig
		wantSubstr string
	}{
		{
			name:       "missing-external-hostname",
			cfg:        HTTPConfig{SkipHostNameTest: true},
			wantSubstr: "missing at least one externalhostname",
		},
		{
			name: "missing-acme-email",
			cfg: HTTPConfig{
				SkipHostNameTest: true,
				ExternalHostName: []string{"example.com"},
			},
			wantSubstr: "missing http.acme.email",
		},
		{
			name: "missing-acme-diskcache",
			cfg: HTTPConfig{
				SkipHostNameTest: true,
				ExternalHostName: []string{"example.com"},
				ACME:             HTTPACMEConfig{Email: "admin@example.com"},
			},
			wantSubstr: "missing http.acme.diskcache",
		},
		{
			name: "valid-static-cert",
			cfg: HTTPConfig{
				SkipHostNameTest: true,
				ExternalHostName: []string{"example.com"},
				StaticCert: HTTPStaticCertConfig{
					SSLCertFile:       "/tmp/cert.pem",
					SSLPrivateKeyFile: "/tmp/key.pem",
				},
			},
			wantSubstr: "",
		},
	}

	for i = 0; i < len(testCases); i++ {
		tc = testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			err = tc.cfg.Verify()
			if len(tc.wantSubstr) == 0 {
				if !errors.Is(err, nil) {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}

			if errors.Is(err, nil) {
				t.Fatalf("expected error containing %q", tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantSubstr, err.Error())
			}
		})
	}
}

func writeTempConfig(t *testing.T, yamlBody string) string {
	var (
		dir  string
		path string
		err  error
	)

	t.Helper()

	dir = t.TempDir()
	path = filepath.Join(dir, "config.yaml")
	err = os.WriteFile(path, []byte(yamlBody), 0o600)
	if !errors.Is(err, nil) {
		t.Fatalf("failed writing temp config file: %v", err)
	}

	return path
}
