package serverconfig

import (
	"fmt"
	"net"
	"net/url"
)

type MySQLDatabase struct {
	Server        string         `yaml:"server" env:"DBSERVER"`
	User          string         `yaml:"user" env:"DBUSER"`
	Password      string         `yaml:"password" env:"DBPASS"`
	DB            string         `yaml:"db" env:"DBNAME"`
	Params        map[string]any `yaml:"params"`
	ConnectString string         `yaml:"connect_string" env:"DBCONNECT"`
}

// Verify checks for necessary parameters to connect to a MySQL source and will construct
// the ConnectString if that wasn't supplied in the configuration YAML.  It is recommended that
// at least parseTime=true should be supplied in Params:
//
//	...
//	db: mydatabase
//	params:
//	  - parseTime: true
func (cfg *MySQLDatabase) Verify() error {
	var err error

	if len(cfg.ConnectString) == 0 {
		if len(cfg.Password) == 0 {
			return fmt.Errorf("missing Database Password (or DBPASS environment variable)")
		}
		if len(cfg.User) == 0 {
			return fmt.Errorf("missing Database User (or DBUSER environment variable)")
		}
		if len(cfg.Server) == 0 {
			return fmt.Errorf("missing Database Server (or DBSERVER environment variable)")
		}
		_, _, err = net.SplitHostPort(cfg.Server)
		if err != nil {
			return fmt.Errorf("database server should specify a port: %w", err)
		}
		cfg.ConnectString = cfg.User + ":" + cfg.Password + "@tcp(" + cfg.Server + ")/" + cfg.DB

		if len(cfg.Params) > 0 {
			vals := url.Values{}
			for k, v := range cfg.Params {
				vals.Add(k, fmt.Sprintf("%v", v))
			}
			cfg.ConnectString += "?" + vals.Encode()
		}
	}
	return nil
}

type PostgresDatabase struct {
	Server        string         `yaml:"server" env:"DBSERVER"`
	User          string         `yaml:"user" env:"DBUSER"`
	Password      string         `yaml:"password" env:"DBPASS"`
	DB            string         `yaml:"db" env:"DBNAME"`
	Params        map[string]any `yaml:"params"`
	ConnectString string         `yaml:"connect_string" env:"DBCONNECT"`
}

// Verify checks for necessary parameters to connect to a Postgres source and will construct
// the ConnectString if that wasn't supplied in the configuration YAML.
func (cfg *PostgresDatabase) Verify() error {
	var err error

	if len(cfg.ConnectString) == 0 {
		if len(cfg.Password) == 0 {
			return fmt.Errorf("missing Database Password (or DBPASS environment variable)")
		}
		if len(cfg.User) == 0 {
			return fmt.Errorf("missing Database User (or DBUSER environment variable)")
		}
		if len(cfg.Server) == 0 {
			return fmt.Errorf("missing Database Server (or DBSERVER environment variable)")
		}
		_, _, err = net.SplitHostPort(cfg.Server)
		if err != nil {
			return fmt.Errorf("database server should specify a port: %w", err)
		}
		cfg.ConnectString = fmt.Sprintf("postgres://%s:%s@%s/%s", cfg.User, cfg.Password, cfg.Server, cfg.DB)

		if len(cfg.Params) > 0 {
			vals := url.Values{}
			for k, v := range cfg.Params {
				vals.Add(k, fmt.Sprintf("%v", v))
			}
			cfg.ConnectString += "?" + vals.Encode()
		}
	}
	return nil
}
