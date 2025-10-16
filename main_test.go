package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
)

// ========================================
// Webhook Signature Verification Tests
// ========================================

func TestVerifySignature_Valid(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"action":"queued"}`)

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !verifySignature(payload, signature, secret) {
		t.Error("Valid signature was rejected")
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"action":"queued"}`)
	signature := "sha256=invalid_signature_here"

	if verifySignature(payload, signature, secret) {
		t.Error("Invalid signature was accepted")
	}
}

func TestVerifySignature_WrongPrefix(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"action":"queued"}`)
	signature := "sha1=somesignature"

	if verifySignature(payload, signature, secret) {
		t.Error("Signature with wrong prefix was accepted")
	}
}

func TestVerifySignature_MissingPrefix(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"action":"queued"}`)
	signature := "justasignature"

	if verifySignature(payload, signature, secret) {
		t.Error("Signature without prefix was accepted")
	}
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	secret := "test-secret"
	wrongSecret := "wrong-secret"
	payload := []byte(`{"action":"queued"}`)

	// Generate signature with correct secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Verify with wrong secret
	if verifySignature(payload, signature, wrongSecret) {
		t.Error("Signature verified with wrong secret")
	}
}

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
// HTTP Handler Tests
// ========================================

func setupTestOrchestrator() *Orchestrator {
	config := Config{
		GitHubPAT:     "test-token",
		GitHubOrg:     "test-org",
		GitHubRepo:    "test-repo",
		WebhookSecret: "test-secret",
		Port:          "8080",
		PoolSize:      2,
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

func TestWebhookHandler_MissingSignature(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	payload := []byte(`{"action":"queued"}`)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(payload))
	w := httptest.NewRecorder()

	orchestrator.webhookHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestWebhookHandler_InvalidSignature(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	payload := []byte(`{"action":"queued"}`)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	w := httptest.NewRecorder()

	orchestrator.webhookHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestWebhookHandler_ValidSignature(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	payload := []byte(`{"action":"queued","workflow_job":{"id":123,"status":"queued"}}`)

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(orchestrator.config.WebhookSecret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	w := httptest.NewRecorder()

	orchestrator.webhookHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	orchestrator.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response struct {
		Status  string `json:"status"`
		VMs     int    `json:"vms"`
		Ready   int    `json:"ready"`
		Running int    `json:"running"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response.Status)
	}

	if response.VMs != orchestrator.config.PoolSize {
		t.Errorf("Expected %d VMs, got %d", orchestrator.config.PoolSize, response.VMs)
	}
}

func TestRunnerConfigHandler_VMNotFound(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	req := httptest.NewRequest("GET", "/api/runner-config/nonexistent", nil)
	w := httptest.NewRecorder()

	// Setup mux to handle path variables
	router := mux.NewRouter()
	router.HandleFunc("/api/runner-config/{vmName}", orchestrator.runnerConfigHandler)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestRunnerConfigHandler_Success(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	// Create a VM slot
	slot := &VMSlot{
		Name:        "runner-1",
		State:       StateReady,
		RunnerToken: "test-token-123",
	}
	orchestrator.vmPool[0] = slot

	req := httptest.NewRequest("GET", "/api/runner-config/runner-1", nil)
	w := httptest.NewRecorder()

	// Setup mux to handle path variables
	router := mux.NewRouter()
	router.HandleFunc("/api/runner-config/{vmName}", orchestrator.runnerConfigHandler)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var config RunnerConfig
	if err := json.Unmarshal(w.Body.Bytes(), &config); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if config.Token != "test-token-123" {
		t.Errorf("Expected token 'test-token-123', got '%s'", config.Token)
	}

	if config.Name != "runner-1" {
		t.Errorf("Expected name 'runner-1', got '%s'", config.Name)
	}

	if config.Organization != orchestrator.config.GitHubOrg {
		t.Errorf("Expected org '%s', got '%s'", orchestrator.config.GitHubOrg, config.Organization)
	}
}

func TestRunnerCompleteHandler(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	// Create a VM slot
	slot := &VMSlot{
		Name:  "runner-1",
		State: StateRunning,
	}
	orchestrator.vmPool[0] = slot

	req := httptest.NewRequest("POST", "/api/runner-complete/runner-1", nil)
	w := httptest.NewRecorder()

	// Setup mux to handle path variables
	router := mux.NewRouter()
	router.HandleFunc("/api/runner-complete/{vmName}", orchestrator.runnerCompleteHandler)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Note: RecreateVM runs asynchronously, so we can't test the full flow here
	// but we can verify the handler accepted the request
}

// ========================================
// VM Pool Management Tests
// ========================================

func TestNewOrchestrator(t *testing.T) {
	config := Config{
		GitHubPAT:  "test-token",
		GitHubOrg:  "test-org",
		PoolSize:   4,
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

func TestGetEnvOrDefault(t *testing.T) {
	// Test with unset environment variable
	result := getEnvOrDefault("NONEXISTENT_VAR_12345", "default-value")
	if result != "default-value" {
		t.Errorf("Expected 'default-value', got '%s'", result)
	}

	// Note: We can't easily test the case where the env var is set
	// without potentially interfering with other tests
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
	// This test requires mocking the GitHub API, so we skip the actual
	// initialization and just test the structure
	config := Config{
		GitHubPAT:     "test-token",
		GitHubOrg:     "test-org",
		GitHubRepo:    "test-repo",
		WebhookSecret: "test-secret",
		PoolSize:      2,
	}

	vmManager := NewMockVMManager(testLogger())
	orchestrator := NewOrchestrator(config, vmManager, testLogger())

	// We can't test InitializePool directly without mocking the GitHub API
	// but we can verify the structure is correct
	if len(orchestrator.vmPool) != 2 {
		t.Errorf("Expected pool size 2, got %d", len(orchestrator.vmPool))
	}
}

func TestRecreateVM_VMNotFound(t *testing.T) {
	orchestrator := setupTestOrchestrator()

	err := orchestrator.RecreateVM("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent VM, got nil")
	}
}
