package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	GitHub     GitHubConfig     `yaml:"github"`
	Runners    RunnersConfig    `yaml:"runners"`
	HyperV     HyperVConfig     `yaml:"hyperv"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Logging    LoggingConfig    `yaml:"logging"`
	Debug      DebugConfig      `yaml:"debug"`
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
	CacheURL    string   `yaml:"cache_url"`    // Optional: URL to custom cache server (must end with /)
}

// HyperVConfig holds Hyper-V specific configuration
type HyperVConfig struct {
	TemplatePath  string `yaml:"template_path"`
	VMStoragePath string `yaml:"storage_path"`
	VMUsername    string `yaml:"vm_username"`   // PowerShell Direct credentials
	VMPassword    string `yaml:"vm_password"`   // PowerShell Direct credentials
	VMMemoryMB    int    `yaml:"vm_memory_mb"`  // VM memory in MB (default: 4096)
	VMCPUCount    int    `yaml:"vm_cpu_count"`  // VM CPU count (default: 2)
}

// MonitoringConfig holds health monitoring configuration
type MonitoringConfig struct {
	HealthCheckIntervalSeconds int `yaml:"health_check_interval_seconds"` // How often to check health (default: 30)
	CreationTimeoutMinutes     int `yaml:"creation_timeout_minutes"`      // Max time for VM to boot and register (default: 5)
	GracePeriodMinutes         int `yaml:"grace_period_minutes"`          // Grace period before checking GitHub registration (default: 5)
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level     string `yaml:"level"`     // Log level: debug, info, warn, error (default: info)
	Format    string `yaml:"format"`    // Log format: text, json (default: text)
	Directory string `yaml:"directory"` // Directory for log files (default: executable directory)
}

// DebugConfig holds debugging configuration
type DebugConfig struct {
	UseMock bool `yaml:"use_mock"` // Use mock VM manager for development/testing
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
	if config.HyperV.VMMemoryMB == 0 {
		config.HyperV.VMMemoryMB = 4096
	}
	if config.HyperV.VMCPUCount == 0 {
		config.HyperV.VMCPUCount = 2
	}
	if config.Monitoring.HealthCheckIntervalSeconds == 0 {
		config.Monitoring.HealthCheckIntervalSeconds = 30
	}
	if config.Monitoring.CreationTimeoutMinutes == 0 {
		config.Monitoring.CreationTimeoutMinutes = 5
	}
	if config.Monitoring.GracePeriodMinutes == 0 {
		config.Monitoring.GracePeriodMinutes = 5
	}
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "text"
	}
	if config.Logging.Directory == "" {
		// Default to executable directory
		exePath, err := os.Executable()
		if err == nil {
			config.Logging.Directory = filepath.Dir(exePath)
		} else {
			// Fallback to current working directory
			cwd, err := os.Getwd()
			if err == nil {
				config.Logging.Directory = cwd
			}
		}
	}

	// Get current working directory for default paths
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

	// Validate cache URL if provided
	if config.Runners.CacheURL != "" && !strings.HasSuffix(config.Runners.CacheURL, "/") {
		return nil, fmt.Errorf("runners.cache_url must end with a trailing slash")
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
