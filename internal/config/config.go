package config

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Fabric FabricConfig `mapstructure:"fabric"`
	Log    LogConfig    `mapstructure:"log"`
}

// ServerConfig holds the HTTP/gRPC-Web proxy server configuration.
type ServerConfig struct {
	ListenAddr      string   `mapstructure:"listenAddr"`
	ShutdownTimeout int      `mapstructure:"shutdownTimeout"` // In seconds
	AllowedOrigins  []string `mapstructure:"allowedOrigins"`  // For CORS and WebSocket
}

// FabricConfig holds the configuration for connecting to the Fabric Gateway.
type FabricConfig struct {
	GatewayAddress string    `mapstructure:"gatewayAddress"`
	Hostname       string    `mapstructure:"hostname"`
	Tls            TlsConfig `mapstructure:"tls"`
}

// TlsConfig defines the paths to certificates and keys for mTLS.
type TlsConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	CaCertPath     string `mapstructure:"caCertPath"`
	ClientCertPath string `mapstructure:"clientCertPath"`
	ClientKeyPath  string `mapstructure:"clientKeyPath"`
}

// LogConfig defines the logging configuration.
type LogConfig struct {
	Level  string `mapstructure:"level"`  // e.g., "debug", "info", "warn", "error"
	Format string `mapstructure:"format"` // e.g., "text", "json"
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig() (config Config, err error) {
	// --- Set up Viper ---
	viper.AddConfigPath(".")      // Look for config in the current directory
	viper.SetConfigName("config") // Look for a file named "config.yaml"
	viper.SetConfigType("yaml")
	viper.SetConfigName(".env") // Also look for a file named ".env"
	viper.SetConfigType("env")

	// --- Set Defaults ---
	viper.SetDefault("server.listenAddr", "localhost:8080")
	viper.SetDefault("server.shutdownTimeout", 5)
	viper.SetDefault("server.allowedOrigins", []string{"*"}) // For development
	viper.SetDefault("fabric.gatewayAddress", "localhost:7051")
	viper.SetDefault("fabric.tls.enabled", true)
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "text")

	_ = viper.ReadInConfig()

	viper.AutomaticEnv()
	viper.SetEnvPrefix("PROXY")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err = viper.Unmarshal(&config)
	if err != nil {
		return
	}

	err = config.validate()
	return
}

func (c *Config) validate() error {
	if c.Server.ListenAddr == "" {
		return errors.New("config error: server.listenAddr cannot be empty")
	}
	if c.Fabric.GatewayAddress == "" {
		return errors.New("config error: fabric.gatewayAddress cannot be empty")
	}
	if c.Fabric.Tls.Enabled {
		if c.Fabric.Tls.CaCertPath == "" || c.Fabric.Tls.ClientCertPath == "" ||
			c.Fabric.Tls.ClientKeyPath == "" {
			return errors.New(
				"config error: if TLS is enabled, caCertPath, clientCertPath, and clientKeyPath must be provided",
			)
		}
		if _, err := os.Stat(c.Fabric.Tls.CaCertPath); err != nil {
			return errors.New(
				"config error: CA certificate file not found at path: " + c.Fabric.Tls.CaCertPath,
			)
		}
		if _, err := os.Stat(c.Fabric.Tls.ClientCertPath); err != nil {
			return errors.New(
				"config error: client certificate file not found at path: " + c.Fabric.Tls.ClientCertPath,
			)
		}
		if _, err := os.Stat(c.Fabric.Tls.ClientKeyPath); err != nil {
			return errors.New(
				"config error: client key file not found at path: " + c.Fabric.Tls.ClientKeyPath,
			)
		}
	}
	return nil
}
