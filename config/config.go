package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"focalors-go/slogger"
)

var logger = slogger.New("config")

// Config holds all configuration for the application
type Config struct {
	App    AppConfig    `mapstructure:"app"`
	Yunzai YunzaiConfig `mapstructure:"yunzai"`
	Wechat WechatConfig `mapstructure:"wechat"`
	Redis  RedisConfig  `mapstructure:"redis"`
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	Debug    bool   `mapstructure:"debug"`
	DataDir  string `mapstructure:"dataDir"`
	LogLevel string `mapstructure:"logLevel"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type YunzaiConfig struct {
	Server string `mapstructure:"server"`
	Admin  string `mapstructure:"admin"`
}

type WechatConfig struct {
	Server string `mapstructure:"server"`
	SubURL string `mapstructure:"subURL"`
	Token  string `mapstructure:"token"`
}

// LoadConfig loads the configuration from the specified file
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set default configuration values
	// setDefaults(v)

	// Set configuration file settings
	v.SetConfigType("toml")

	// If config path is provided, use it
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Otherwise, look for config in the default locations
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("$HOME/.focalors-go")
		v.AddConfigPath("/etc/focalors-go")
	}

	// Try to read the config file
	if err := v.ReadInConfig(); err != nil {
		// If the config file wasn't found, initialize and create one
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("Config file not found, creating default configuration")
			if configPath != "" {
				return createDefaultConfig(v, filepath.Dir(configPath))
			}
			return createDefaultConfig(v, ".")
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse the config into our struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	if config.Wechat.Token == "" {
		return nil, fmt.Errorf("wechat token is required")
	}

	return &config, nil
}

// setDefaults sets default values for configuration
// func setDefaults(v *viper.Viper) {
// 	v.SetDefault("app", AppConfig{
// 		Debug:    false,
// 		DataDir:  "$HOME/.focalors-go",
// 		LogLevel: "info",
// 	})

// 	v.Set("redis", RedisConfig{
// 		Addr:     "localhost:6379",
// 		Password: "",
// 		DB:       0,
// 	})

// 	v.Set("yunzai", YunzaiConfig{
// 		Server: "ws://localhost:2536/GSUIDCore",
// 		Admin:  "",
// 	})

// 	v.Set("wechat", WechatConfig{
// 		Server: "http://localhost:1239",
// 		SubURL: "ws://localhost:1239/ws/GetSyncMsg",
// 	})
// }

// createDefaultConfig creates a default configuration file if none exists
func createDefaultConfig(v *viper.Viper, configDir string) (*Config, error) {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating config directory: %w", err)
	}

	// Create default config file
	configFile := filepath.Join(configDir, "config.toml")
	if err := v.WriteConfigAs(configFile); err != nil {
		return nil, fmt.Errorf("error creating default config file: %w", err)
	}

	logger.Info("Created default config file", "path", configFile)

	// Parse the config into our struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	return &config, nil
}
