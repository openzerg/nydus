package config

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Cerebrate CerebrateConfig
}

type ServerConfig struct {
	Host string
	Port int
}

type DatabaseConfig struct {
	Path string
}

type CerebrateConfig struct {
	Host string
	Port int
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 15318,
		},
		Database: DatabaseConfig{
			Path: "/var/lib/nydus/nydus.db",
		},
		Cerebrate: CerebrateConfig{
			Host: "10.0.0.1",
			Port: 15319,
		},
	}
}
