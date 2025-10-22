package orchestrator

import (
	"log/slog"
	"os"
	"testing"

	"hyperv-runner-pool/pkg/config"
	"hyperv-runner-pool/pkg/github"
	"hyperv-runner-pool/pkg/vmmanager"
)

// testLogger creates a logger for tests (discards output)
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
}

func setupTestOrchestrator() *Orchestrator {
	cfg := config.Config{
		GitHub: config.GitHubConfig{
			AppID:             123456,
			AppPrivateKeyPath: "/mock/path/to/key.pem",
			Org:               "test-org",
			Repo:              "test-repo",
		},
		Runners: config.RunnersConfig{
			PoolSize: 2,
		},
		Debug: config.DebugConfig{
			UseMock: true, // Enable mock mode for tests
		},
	}

	vmManager := vmmanager.NewMockVMManager(testLogger())
	ghClient := github.NewClient(cfg, testLogger())
	orchestrator := New(cfg, vmManager, ghClient, testLogger())

	// Initialize VM slots for testing
	for i := 0; i < orchestrator.config.Runners.PoolSize; i++ {
		orchestrator.vmPool[i] = &vmmanager.VMSlot{
			Name:  "runner-" + string(rune('0'+i+1)),
			State: vmmanager.StateEmpty,
		}
	}

	return orchestrator
}

func TestNewOrchestrator(t *testing.T) {
	cfg := config.Config{
		GitHub: config.GitHubConfig{
			AppID:             123456,
			AppPrivateKeyPath: "/mock/path/to/key.pem",
			Org:               "test-org",
		},
		Runners: config.RunnersConfig{
			PoolSize: 4,
		},
		Debug: config.DebugConfig{
			UseMock: true, // Enable mock mode for tests
		},
	}

	vmManager := vmmanager.NewMockVMManager(testLogger())
	ghClient := github.NewClient(cfg, testLogger())
	orchestrator := New(cfg, vmManager, ghClient, testLogger())

	if orchestrator == nil {
		t.Fatal("Failed to create orchestrator")
	}

	if len(orchestrator.vmPool) != cfg.Runners.PoolSize {
		t.Errorf("Expected pool size %d, got %d", cfg.Runners.PoolSize, len(orchestrator.vmPool))
	}

	if orchestrator.config.GitHub.AppID != cfg.GitHub.AppID {
		t.Error("Config not properly set")
	}
}

func TestRecreateVM_VMNotFound(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	err := orchestrator.RecreateVM("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent VM, got nil")
	}
}

func TestOrchestratorInitializePool(t *testing.T) {
	cfg := config.Config{
		GitHub: config.GitHubConfig{
			AppID:             123456,
			AppPrivateKeyPath: "/mock/path/to/key.pem",
			Org:               "test-org",
			Repo:              "test-repo",
		},
		Runners: config.RunnersConfig{
			PoolSize: 2,
		},
		Debug: config.DebugConfig{
			UseMock: true, // Enable mock mode for tests
		},
	}

	vmManager := vmmanager.NewMockVMManager(testLogger())
	ghClient := github.NewClient(cfg, testLogger())
	orchestrator := New(cfg, vmManager, ghClient, testLogger())

	// Verify the pool structure is correct
	if len(orchestrator.vmPool) != 2 {
		t.Errorf("Expected pool size 2, got %d", len(orchestrator.vmPool))
	}
}
