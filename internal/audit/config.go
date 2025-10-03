package audit

import "flag"

type Config struct {
	LogLevel      string `env:"AUDIT_LOG_LEVEL,default=INFO"`
	StartURL      string `env:"AUDIT_START_URL,default="`
	Agent         string `env:"AUDIT_AGENT,default=agent"`
	ValidSchemes  string `env:"AUDIT_VALID_SCHEMES,default=https"`
	RespectRobots bool   `env:"AUDIT_RESPECT_ROBOTS,default=TRUE"`
	MaxWorkers    int    `env:"AUDIT_MAX_WORKERS,default=10"`
	MaxDepth      int    `env:"AUDIT_MAX_DEPTH,default=2"`
}

func AddFlags(config Config, fs *flag.FlagSet) {
	fs.StringVar(&config.LogLevel, "AUDIT_LOG_LEVEL", "INFO", "The log level")
	fs.StringVar(&config.StartURL, "AUDIT_START_URL", "", "The start URL")
	fs.StringVar(&config.ValidSchemes, "AUDIT_VALID_SCHEMES", "https", "Comma-separated list of values for valid schemes")
	fs.BoolVar(&config.RespectRobots, "AUDIT_RESPECT_ROBOTS", true, "Whether to respsect the robots.txt file")
	fs.IntVar(&config.MaxWorkers, "AUDIT_MAX_WORKERS", 10, "Maximum number of worker routines")
	fs.IntVar(&config.MaxDepth, "AUDIT_MAX_DEPTH", 2, "The maximum depth to traverse through links")
}
