package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the proxy server configuration
type Config struct {
	ListenAddress        string `json:"listen_address"`
	ListenPort          int    `json:"listen_port"`
	ConcurrencyModel    string `json:"concurrency_model"`
	ThreadPoolSize      int    `json:"thread_pool_size"`
	LogFilePath         string `json:"log_file_path"`
	LogMaxSizeMB        int    `json:"log_max_size_mb"`
	BlockedDomainsFile  string `json:"blocked_domains_file"`
	EnableCaching       bool   `json:"enable_caching"`
	CacheMaxEntries     int    `json:"cache_max_entries"`
	EnableConnectTunnel bool   `json:"enable_connect_tunneling"`
	AuthToken           string `json:"authentication_token"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		ListenAddress:       "0.0.0.0",
		ListenPort:          8888,
		ConcurrencyModel:    "thread_per_connection",
		ThreadPoolSize:      10,
		LogFilePath:         "proxy.log",
		LogMaxSizeMB:        100,
		BlockedDomainsFile:  "config/blocked_domains.txt",
		EnableCaching:       false,
		CacheMaxEntries:     1000,
		EnableConnectTunnel: false,
		AuthToken:           "",
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.ListenPort < 1 || c.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be between 1 and 65535")
	}

	if c.ConcurrencyModel != "thread_per_connection" && c.ConcurrencyModel != "thread_pool" {
		return fmt.Errorf("concurrency_model must be 'thread_per_connection' or 'thread_pool'")
	}

	if c.ConcurrencyModel == "thread_pool" && c.ThreadPoolSize < 1 {
		return fmt.Errorf("thread_pool_size must be at least 1")
	}

	if c.LogMaxSizeMB < 1 {
		return fmt.Errorf("log_max_size_mb must be at least 1")
	}

	if c.EnableCaching && c.CacheMaxEntries < 1 {
		return fmt.Errorf("cache_max_entries must be at least 1 when caching is enabled")
	}

	return nil
}

// LoadConfigFromINI loads configuration from a simple INI-like format
// Format: key=value (one per line, # for comments)
func LoadConfigFromINI(path string) (*Config, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "listen_address":
			config.ListenAddress = value
		case "listen_port":
			if port, err := strconv.Atoi(value); err == nil {
				config.ListenPort = port
			}
		case "concurrency_model":
			config.ConcurrencyModel = value
		case "thread_pool_size":
			if size, err := strconv.Atoi(value); err == nil {
				config.ThreadPoolSize = size
			}
		case "log_file_path":
			config.LogFilePath = value
		case "log_max_size_mb":
			if size, err := strconv.Atoi(value); err == nil {
				config.LogMaxSizeMB = size
			}
		case "blocked_domains_file":
			config.BlockedDomainsFile = value
		case "enable_caching":
			config.EnableCaching = strings.ToLower(value) == "true"
		case "cache_max_entries":
			if size, err := strconv.Atoi(value); err == nil {
				config.CacheMaxEntries = size
			}
		case "enable_connect_tunneling":
			config.EnableConnectTunnel = strings.ToLower(value) == "true"
		case "authentication_token":
			config.AuthToken = value
		}
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

