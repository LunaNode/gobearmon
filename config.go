package gobearmon

import "gopkg.in/gcfg.v1"

import "log"

type DefaultConfig struct {
	Debug bool
	Password string
}

type ControllerConfig struct {
	Addr string
	Database []string
	Confirmations int
}

type WorkerConfig struct {
	ViewAddr string
	NumThreads int
}

type SmtpConfig struct {
	Host string
	Port int
	From string
	Username string
	Password string
	Admin string
}

type DNSConfig struct {
	Server string
}

type TwilioConfig struct {
	AccountSid string
	AuthToken string
	From string
}

type ViewServerConfig struct {
	Addr string
	Controller []string
}

type Config struct {
	Default DefaultConfig
	Controller ControllerConfig
	Worker WorkerConfig
	Smtp SmtpConfig
	DNS DNSConfig
	Twilio TwilioConfig
	ViewServer ViewServerConfig
}

func LoadConfig(cfgPath string) *Config {
	var cfg Config
	err := gcfg.ReadFileInto(&cfg, cfgPath)
	if err != nil {
		log.Fatalf("Error while reading configuration: %s", err.Error())
	}
	return &cfg
}
