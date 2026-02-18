# Server Config

This package provides a configuration loader for Go applications. It supports reading configuration from YAML files and overriding values with environment variables.

## Features

- **YAML Configuration**: Load configuration from YAML files.
- **Environment Variable Overrides**: Override configuration values using environment variables.
- **Verification**: Implement the `Verifier` interface to validate configuration structs.
- **Supported Types**: Supports basic types (string, bool, int, uint, float), `time.Duration`, and slices of strings.

## Usage

### Define Configuration Structs

Define your configuration structs with `yaml` and `env` tags.

```go
type Config struct {
    Logging  LoggingConfig `yaml:"logging"`
    Database MySQLDatabase `yaml:"database"`
    // ... other config sections
}

type MySQLDatabase struct {
    Server        string         `yaml:"server" env:"DBSERVER"`
    User          string         `yaml:"user" env:"DBUSER"`
    Password      string         `yaml:"password" env:"DBPASS"`
    DB            string         `yaml:"db"`
    Params        map[string]any `yaml:"params"`
    ConnectString string         `yaml:"connect_string" env:"DBCONNECT"`
}
```

### Load Configuration

Use the `Read` function to load the configuration.

```go
var cfg Config
err := serverconfig.Read("config.yaml", &cfg)
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}
```

### Verification

Implement the `Verifier` interface to add custom validation logic.

```go
func (cfg *MySQLDatabase) Verify() error {
    if len(cfg.ConnectString) == 0 {
        // ... validation logic
    }
    return nil
}
```

## Environment Variables

You can override configuration values by setting the environment variable specified in the `env` tag.

Example:
```bash
export DBSERVER="localhost:3306"
export DBUSER="admin"
export DBPASS="secret"
```
