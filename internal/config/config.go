package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Fabric FabricConfig `mapstructure:"fabric"`
	Log    LogConfig    `mapstructure:"log"`
}

type ServerConfig struct {
	ListenAddr      string   `mapstructure:"listenAddr"`
	AllowedOrigins  []string `mapstructure:"allowedOrigins"`
	ShutdownTimeout int      `mapstructure:"shutdownTimeout"`
}

type FabricConfig struct {
	GatewayAddress string    `mapstructure:"gatewayAddress"`
	Hostname       string    `mapstructure:"hostname"`
	TLS            TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	CaCertPath     string `mapstructure:"caCertPath"`
	ClientCertPath string `mapstructure:"clientCertPath"`
	ClientKeyPath  string `mapstructure:"clientKeyPath"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func LoadConfig() (*Config, error) {
	var cfg Config

	// Server
	viper.BindEnv("server.listenAddr", "SERVER_LISTEN_ADDR")
	viper.BindEnv("server.allowedOrigins", "SERVER_ALLOWED_ORIGINS")

	// Fabric
	viper.BindEnv("fabric.gatewayAddress", "FABRIC_GATEWAY_ADDRESS")
	viper.BindEnv("fabric.hostname", "FABRIC_HOSTNAME")

	// Fabric TLS
	viper.BindEnv("fabric.tls.enabled", "FABRIC_TLS_ENABLED")
	viper.BindEnv("fabric.tls.caCertPath", "FABRIC_TLS_CA_CERT_PATH")
	viper.BindEnv("fabric.tls.clientCertPath", "FABRIC_TLS_CLIENT_CERT_PATH")
	viper.BindEnv("fabric.tls.clientKeyPath", "FABRIC_TLS_CLIENT_KEY_PATH")

	// Log
	viper.BindEnv("log.level", "LOG_LEVEL")
	viper.BindEnv("log.format", "LOG_FORMAT")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/grpcweb-proxy/")
	viper.AddConfigPath(".") // Para desarrollo local

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
