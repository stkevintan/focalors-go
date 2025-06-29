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
	App     AppConfig     `mapstructure:"app"`
	Yunzai  YunzaiConfig  `mapstructure:"yunzai"`
	Wechat  WechatConfig  `mapstructure:"wechat"`
	Jiadan  JiadanConfig  `mapstructure:"jiadan"`
	OpenAI  OpenAIConfig  `mapstructure:"openai"`
	Weather WeatherConfig `mapstructure:"weather"`
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	Debug    bool        `mapstructure:"debug"`
	LogLevel string      `mapstructure:"logLevel"`
	Admin    string      `mapstructure:"admin"`
	SyncCron string      `mapstructure:"syncCron"`
	Redis    RedisConfig `mapstructure:"redis"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type YunzaiConfig struct {
	Server string `mapstructure:"server"`
}

type WechatConfig struct {
	Server string `mapstructure:"server"`
	SubURL string `mapstructure:"subURL"`
	Token  string `mapstructure:"token"`
}

type OpenAIConfig struct {
	APIKey     string `mapstructure:"apiKey"`
	Endpoint   string `mapstructure:"endpoint"`
	APIVersion string `mapstructure:"apiVersion"`
	Deployment string `mapstructure:"deployment"`
}

type WeatherConfig struct {
	Key string `mapstructure:"key"`
}

type JiadanConfig struct {
	MaxSyncCount int `mapstructure:"maxSyncCount"`
}

// LoadConfig loads the configuration from the specified file
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set default configuration values
	setDefaults(v)

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

	if config.Jiadan.MaxSyncCount < 1 {
		return nil, fmt.Errorf("jiadan max sync count must be greater than 0")
	}

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.debug", false)
	v.SetDefault("app.logLevel", "info")
	v.SetDefault("app.admin", "")
	v.SetDefault("app.syncCron", "*/60 8-23 * * *")

	// Redis defaults
	v.SetDefault("app.redis.addr", "localhost:6379")
	v.SetDefault("app.redis.password", "")
	v.SetDefault("app.redis.db", 0)

	// Yunzai defaults
	v.SetDefault("yunzai.server", "ws://localhost:2536/GSUIDCore")

	// Wechat defaults
	v.SetDefault("wechat.server", "http://localhost:1239")
	v.SetDefault("wechat.subURL", "ws://localhost:1239/ws/GetSyncMsg")
	v.SetDefault("wechat.token", "")

	// Jiadan defaults
	v.SetDefault("jiadan.maxSyncCount", 4)

	v.SetDefault("openai.apiVersion", "2025-03-01-preview")
}

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
