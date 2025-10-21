package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"
)

// ========================================
// Mock VM Manager Tests
// ========================================

// testLogger creates a logger for tests (discards output)
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
}

func TestMockVMManager_CreateVM(t *testing.T) {
	manager := NewMockVMManager(testLogger())
	slot := &VMSlot{
		Name:  "test-runner-1",
		State: StateEmpty,
	}

	err := manager.CreateVM(slot)
	if err != nil {
		t.Fatalf("Failed to create VM: %v", err)
	}

	// Verify VM was added to simulated VMs
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if _, exists := manager.simulatedVMs[slot.Name]; !exists {
		t.Error("VM was not added to simulated VMs map")
	}
}

func TestMockVMManager_DestroyVM(t *testing.T) {
	manager := NewMockVMManager(testLogger())
	slot := &VMSlot{
		Name:  "test-runner-1",
		State: StateRunning,
	}

	// Create VM first
	manager.CreateVM(slot)

	// Verify it exists
	manager.mu.Lock()
	if _, exists := manager.simulatedVMs[slot.Name]; !exists {
		t.Fatal("VM was not created")
	}
	manager.mu.Unlock()

	// Destroy VM
	err := manager.DestroyVM(slot)
	if err != nil {
		t.Fatalf("Failed to destroy VM: %v", err)
	}

	// Verify it was removed
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if _, exists := manager.simulatedVMs[slot.Name]; exists {
		t.Error("VM was not removed from simulated VMs map")
	}
}

func TestMockVMManager_GetVMState(t *testing.T) {
	manager := NewMockVMManager(testLogger())
	slot := &VMSlot{
		Name:  "test-runner-1",
		State: StateEmpty,
	}

	// Create VM first
	err := manager.CreateVM(slot)
	if err != nil {
		t.Fatalf("Failed to create VM: %v", err)
	}

	// Get VM state
	state, err := manager.GetVMState(slot.Name)
	if err != nil {
		t.Fatalf("Failed to get VM state: %v", err)
	}

	if state != "Running" {
		t.Errorf("Expected state 'Running', got '%s'", state)
	}

	// Test nonexistent VM
	_, err = manager.GetVMState("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent VM, got nil")
	}
}

func TestMockVMManager_InjectConfig(t *testing.T) {
	manager := NewMockVMManager(testLogger())
	config := RunnerConfig{
		Token:        "test-token",
		Organization: "test-org",
		Repository:   "test-repo",
		Name:         "runner-1",
		Labels:       "self-hosted,Windows,X64,ephemeral",
	}

	err := manager.InjectConfig("/fake/path.vhdx", config)
	if err != nil {
		t.Fatalf("Failed to inject config: %v", err)
	}
}

func TestMockVMManager_RunPowerShell(t *testing.T) {
	manager := NewMockVMManager(testLogger())
	output, err := manager.RunPowerShell("Get-VM")

	if err != nil {
		t.Fatalf("RunPowerShell failed: %v", err)
	}

	if output != "mock output" {
		t.Errorf("Expected 'mock output', got '%s'", output)
	}
}

// ========================================
// Orchestrator Tests
// ========================================

func setupTestOrchestrator() *Orchestrator {
	config := Config{
		GitHubPAT:  "test-token",
		GitHubOrg:  "test-org",
		GitHubRepo: "test-repo",
		PoolSize:   2,
	}

	vmManager := NewMockVMManager(testLogger())
	orchestrator := NewOrchestrator(config, vmManager, testLogger())

	// Initialize VM slots for testing
	for i := 0; i < orchestrator.config.PoolSize; i++ {
		orchestrator.vmPool[i] = &VMSlot{
			Name:  "runner-" + string(rune('0'+i+1)),
			State: StateEmpty,
		}
	}

	return orchestrator
}

func TestNewOrchestrator(t *testing.T) {
	config := Config{
		GitHubPAT: "test-token",
		GitHubOrg: "test-org",
		PoolSize:  4,
	}

	vmManager := NewMockVMManager(testLogger())
	orchestrator := NewOrchestrator(config, vmManager, testLogger())

	if orchestrator == nil {
		t.Fatal("Failed to create orchestrator")
	}

	if len(orchestrator.vmPool) != config.PoolSize {
		t.Errorf("Expected pool size %d, got %d", config.PoolSize, len(orchestrator.vmPool))
	}

	if orchestrator.config.GitHubPAT != config.GitHubPAT {
		t.Error("Config not properly set")
	}
}

func TestVMSlot_StateTransitions(t *testing.T) {
	slot := &VMSlot{
		Name:  "test-runner",
		State: StateEmpty,
	}

	// Test state transitions
	states := []VMState{StateCreating, StateReady, StateRunning, StateDestroying, StateEmpty}

	for _, expectedState := range states {
		slot.mu.Lock()
		slot.State = expectedState
		actualState := slot.State
		slot.mu.Unlock()

		if actualState != expectedState {
			t.Errorf("State transition failed: expected %s, got %s", expectedState, actualState)
		}
	}
}

func TestRecreateVM_VMNotFound(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	err := orchestrator.RecreateVM("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent VM, got nil")
	}
}

// ========================================
// RunnerConfig Tests
// ========================================

func TestRunnerConfig_JSONSerialization(t *testing.T) {
	config := RunnerConfig{
		Token:        "test-token-123",
		Organization: "test-org",
		Repository:   "test-repo",
		Name:         "runner-1",
		Labels:       "self-hosted,Windows,X64,ephemeral",
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Deserialize from JSON
	var decoded RunnerConfig
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Verify fields
	if decoded.Token != config.Token {
		t.Errorf("Token mismatch: expected '%s', got '%s'", config.Token, decoded.Token)
	}
	if decoded.Organization != config.Organization {
		t.Errorf("Organization mismatch: expected '%s', got '%s'", config.Organization, decoded.Organization)
	}
	if decoded.Name != config.Name {
		t.Errorf("Name mismatch: expected '%s', got '%s'", config.Name, decoded.Name)
	}
}

// ========================================
// Utility Function Tests
// ========================================

func TestGetEnvOrDefault(t *testing.T) {
	// Test with unset environment variable
	result := getEnvOrDefault("NONEXISTENT_VAR_12345", "default-value")
	if result != "default-value" {
		t.Errorf("Expected 'default-value', got '%s'", result)
	}

	// Test with set environment variable
	os.Setenv("TEST_VAR_12345", "actual-value")
	defer os.Unsetenv("TEST_VAR_12345")

	result = getEnvOrDefault("TEST_VAR_12345", "default-value")
	if result != "actual-value" {
		t.Errorf("Expected 'actual-value', got '%s'", result)
	}
}

// ========================================
// Concurrent Operations Tests
// ========================================

func TestConcurrentVMOperations(t *testing.T) {
	manager := NewMockVMManager(testLogger())

	// Create multiple VMs concurrently
	slots := make([]*VMSlot, 10)
	for i := 0; i < 10; i++ {
		slots[i] = &VMSlot{
			Name:  "runner-" + string(rune('0'+i)),
			State: StateEmpty,
		}
	}

	// Create all VMs concurrently
	errChan := make(chan error, 10)
	for _, slot := range slots {
		go func(s *VMSlot) {
			errChan <- manager.CreateVM(s)
		}(slot)
	}

	// Check for errors
	for i := 0; i < 10; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent VM creation failed: %v", err)
		}
	}

	// Verify all VMs were created
	manager.mu.Lock()
	if len(manager.simulatedVMs) != 10 {
		t.Errorf("Expected 10 VMs, got %d", len(manager.simulatedVMs))
	}
	manager.mu.Unlock()

	// Destroy all VMs concurrently
	for _, slot := range slots {
		go func(s *VMSlot) {
			errChan <- manager.DestroyVM(s)
		}(slot)
	}

	// Check for errors
	for i := 0; i < 10; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent VM destruction failed: %v", err)
		}
	}

	// Verify all VMs were destroyed
	manager.mu.Lock()
	if len(manager.simulatedVMs) != 0 {
		t.Errorf("Expected 0 VMs, got %d", len(manager.simulatedVMs))
	}
	manager.mu.Unlock()
}

// ========================================
// Integration-style Tests
// ========================================

func TestOrchestratorInitializePool(t *testing.T) {
	config := Config{
		GitHubPAT:  "test-token",
		GitHubOrg:  "test-org",
		GitHubRepo: "test-repo",
		PoolSize:   2,
	}

	vmManager := NewMockVMManager(testLogger())
	orchestrator := NewOrchestrator(config, vmManager, testLogger())

	// Verify the pool structure is correct
	if len(orchestrator.vmPool) != 2 {
		t.Errorf("Expected pool size 2, got %d", len(orchestrator.vmPool))
	}
}
