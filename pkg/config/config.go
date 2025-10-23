package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	GitHub  GitHubConfig  `yaml:"github"`
	Runners RunnersConfig `yaml:"runners"`
	HyperV  HyperVConfig  `yaml:"hyperv"`
	Debug   DebugConfig   `yaml:"debug"`
}

// GitHubConfig holds GitHub-specific configuration
type GitHubConfig struct {
	AppID             int64  `yaml:"app_id"`
	AppPrivateKeyPath string `yaml:"app_private_key_path"`
	Org               string `yaml:"org"`
	User              string `yaml:"user"` // Alternative to Org for personal accounts
	Repo              string `yaml:"repo"`
}

// GetAccount returns the account name (org or user)
func (c *GitHubConfig) GetAccount() string {
	if c.Org != "" {
		return c.Org
	}
	return c.User
}

// RunnersConfig holds runner pool configuration
type RunnersConfig struct {
	PoolSize    int      `yaml:"pool_size"`
	NamePrefix  string   `yaml:"name_prefix"`
	Labels      []string `yaml:"labels"`       // Custom labels to add to runners
	RunnerGroup string   `yaml:"runner_group"` // Runner group (org-level runners only)
}

// HyperVConfig holds Hyper-V specific configuration
type HyperVConfig struct {
	TemplatePath  string `yaml:"template_path"`
	VMStoragePath string `yaml:"storage_path"`
	VMUsername    string `yaml:"vm_username"` // PowerShell Direct credentials
	VMPassword    string `yaml:"vm_password"` // PowerShell Direct credentials
}

// DebugConfig holds debugging and logging configuration
type DebugConfig struct {
	UseMock      bool   `yaml:"use_mock"`
	LogLevel     string `yaml:"log_level"`
	LogFormat    string `yaml:"log_format"`
	LogDirectory string `yaml:"log_directory"`
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Set defaults
	if config.Runners.PoolSize == 0 {
		config.Runners.PoolSize = 1
	}
	if config.Runners.NamePrefix == "" {
		config.Runners.NamePrefix = "runner-"
	}
	if config.HyperV.VMUsername == "" {
		config.HyperV.VMUsername = "Administrator"
	}
	if config.HyperV.VMPassword == "" {
		config.HyperV.VMPassword = "password"
	}
	if config.Debug.LogLevel == "" {
		config.Debug.LogLevel = "info"
	}
	if config.Debug.LogFormat == "" {
		config.Debug.LogFormat = "text"
	}
	if config.Debug.LogDirectory == "" {
		// Default to executable directory
		exePath, err := os.Executable()
		if err == nil {
			config.Debug.LogDirectory = filepath.Dir(exePath)
		} else {
			// Fallback to current working directory
			cwd, err := os.Getwd()
			if err == nil {
				config.Debug.LogDirectory = cwd
			}
		}
	}

	// Get current working directory for default paths
	if config.HyperV.TemplatePath == "" || config.HyperV.VMStoragePath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}

		if config.HyperV.TemplatePath == "" {
			config.HyperV.TemplatePath = fmt.Sprintf(`%s\vms\templates\runner-template.vhdx`, cwd)
		}
		if config.HyperV.VMStoragePath == "" {
			config.HyperV.VMStoragePath = fmt.Sprintf(`%s\vms\storage`, cwd)
		}
	}

	// Validate required fields (unless in mock mode)
	if !config.Debug.UseMock {
		if config.GitHub.AppID == 0 {
			return nil, fmt.Errorf("github.app_id is required when debug.use_mock is false")
		}
		if config.GitHub.AppPrivateKeyPath == "" {
			return nil, fmt.Errorf("github.app_private_key_path is required when debug.use_mock is false")
		}
		if config.GitHub.GetAccount() == "" {
			return nil, fmt.Errorf("either github.org or github.user is required when debug.use_mock is false")
		}
		// Verify the private key file exists
		if _, err := os.Stat(config.GitHub.AppPrivateKeyPath); err != nil {
			return nil, fmt.Errorf("github app private key file not found at %s: %w", config.GitHub.AppPrivateKeyPath, err)
		}
	} else {
		// Set dummy values for mock mode if not provided
		if config.GitHub.AppID == 0 {
			config.GitHub.AppID = 123456
		}
		if config.GitHub.AppPrivateKeyPath == "" {
			config.GitHub.AppPrivateKeyPath = "/mock/path/to/key.pem"
		}
		if config.GitHub.GetAccount() == "" {
			config.GitHub.Org = "mock-org"
		}
	}

	return &config, nil
}
