package serverconfig

type SMTPConfig struct {
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user" env:"SMTPUSER"`
	Password string `yaml:"password" env:"SMTPPASS"`
	From     string `yaml:"from"`
}
