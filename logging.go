package serverconfig

import (
	"fmt"
	"log/syslog"
)

var (
	logFacilityString2Int = map[string]syslog.Priority{
		"LOG_KERN":     syslog.LOG_KERN,
		"LOG_USER":     syslog.LOG_USER,
		"LOG_MAIL":     syslog.LOG_MAIL,
		"LOG_DAEMON":   syslog.LOG_DAEMON,
		"LOG_AUTH":     syslog.LOG_AUTH,
		"LOG_SYSLOG":   syslog.LOG_SYSLOG,
		"LOG_LPR":      syslog.LOG_LPR,
		"LOG_NEWS":     syslog.LOG_NEWS,
		"LOG_UUCP":     syslog.LOG_UUCP,
		"LOG_CRON":     syslog.LOG_CRON,
		"LOG_AUTHPRIV": syslog.LOG_AUTHPRIV,
		"LOG_FTP":      syslog.LOG_FTP,
		"LOG_LOCAL0":   syslog.LOG_LOCAL0,
		"LOG_LOCAL1":   syslog.LOG_LOCAL1,
		"LOG_LOCAL2":   syslog.LOG_LOCAL2,
		"LOG_LOCAL3":   syslog.LOG_LOCAL3,
		"LOG_LOCAL4":   syslog.LOG_LOCAL4,
		"LOG_LOCAL5":   syslog.LOG_LOCAL5,
		"LOG_LOCAL6":   syslog.LOG_LOCAL6,
		"LOG_LOCAL7":   syslog.LOG_LOCAL7,
	}
	logSeverityString2Int = map[string]syslog.Priority{
		"LOG_EMERG":   syslog.LOG_EMERG,
		"LOG_ALERT":   syslog.LOG_ALERT,
		"LOG_CRIT":    syslog.LOG_CRIT,
		"LOG_ERR":     syslog.LOG_ERR,
		"LOG_WARNING": syslog.LOG_WARNING,
		"LOG_NOTICE":  syslog.LOG_NOTICE,
		"LOG_INFO":    syslog.LOG_INFO,
		"LOG_DEBUG":   syslog.LOG_DEBUG,
	}
)

// LoggingConfig can be used to get configuration necessary to instantiate a syslog logger output.
// Example:
//
//	 serverconfig.Read("xxx.yml", &gc)
//		if !gc.Logging.SyslogEnabled {
//			logger = log.New(os.Stderr, "", log.LstdFlags|log.Lmsgprefix)
//		} else {
//			logger, err = syslog.NewLogger(gc.Logging.Priority, log.Lmsgprefix)
//			if err != nil {
//				log.Fatal(err)
//			}
//		}
type LoggingConfig struct {
	SyslogEnabled bool                `yaml:"syslog_enabled"`
	Syslog        LoggingSyslogConfig `yaml:"syslog"`
}

type LoggingSyslogConfig struct {
	FacilityString string          `yaml:"facility"`
	SeverityString string          `yaml:"severity"`
	priority       syslog.Priority `yaml:"-"`
}

func (cfg *LoggingConfig) Verify() error {
	var (
		facility syslog.Priority
		severity syslog.Priority
		found    bool
	)

	if len(cfg.Syslog.FacilityString) == 0 {
		cfg.Syslog.FacilityString = "LOG_LOCAL5"
	}
	if len(cfg.Syslog.SeverityString) == 0 {
		cfg.Syslog.SeverityString = "LOG_INFO"
	}

	facility, found = logFacilityString2Int[cfg.Syslog.FacilityString]
	if !found {
		return fmt.Errorf("invalid logging facility specified: '%s'", cfg.Syslog.FacilityString)
	}

	severity, found = logSeverityString2Int[cfg.Syslog.SeverityString]
	if !found {
		return fmt.Errorf("invalid logging severity specified: '%s'", cfg.Syslog.SeverityString)
	}

	cfg.Syslog.priority = syslog.Priority(int(facility) | int(severity))
	return nil
}

func (cfg LoggingSyslogConfig) Priority() syslog.Priority {
	return cfg.priority
}
