package vmmanager

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// MockVMManager implements VMManager for testing on non-Windows platforms
type MockVMManager struct {
	simulatedVMs map[string]string
	mu           sync.Mutex
	logger       *slog.Logger
}

// NewMockVMManager creates a new mock VM manager
func NewMockVMManager(logger *slog.Logger) *MockVMManager {
	return &MockVMManager{
		simulatedVMs: make(map[string]string),
		logger:       logger.With("component", "mock"),
	}
}

// CreateVM simulates VM creation
func (m *MockVMManager) CreateVM(slot *VMSlot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate creation delay
	time.Sleep(500 * time.Millisecond)

	m.simulatedVMs[slot.Name] = "Running"
	m.logger.Debug("VM created (simulated)", "vm_name", slot.Name)
	return nil
}

// DestroyVM simulates VM destruction
func (m *MockVMManager) DestroyVM(slot *VMSlot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate destruction delay
	time.Sleep(300 * time.Millisecond)

	delete(m.simulatedVMs, slot.Name)
	m.logger.Debug("VM destroyed (simulated)", "vm_name", slot.Name)
	return nil
}

// GetVMState simulates getting VM state
func (m *MockVMManager) GetVMState(vmName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.simulatedVMs[vmName]
	if !exists {
		return "", fmt.Errorf("VM not found: %s", vmName)
	}
	return state, nil
}

// InjectConfig simulates config injection
func (m *MockVMManager) InjectConfig(vhdxPath string, config RunnerConfig) error {
	m.logger.Debug("Config injected (simulated)", "path", vhdxPath, "vm_name", config.Name)
	return nil
}

// RunPowerShell simulates PowerShell command execution
func (m *MockVMManager) RunPowerShell(command string) (string, error) {
	m.logger.Debug("PowerShell command (simulated)", "command", command)
	return "mock output", nil
}

// CleanupLeftoverResources simulates cleanup
func (m *MockVMManager) CleanupLeftoverResources(namePrefix string) error {
	m.logger.Debug("Cleanup leftover resources (simulated)", "name_prefix", namePrefix)
	return nil
}
