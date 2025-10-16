package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Version information (set by GoReleaser during build)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// ========================================
// Configuration Structures
// ========================================

// Config holds the application configuration
type Config struct {
	GitHubPAT      string
	GitHubOrg      string
	GitHubRepo     string
	WebhookSecret  string
	Port           string
	PoolSize       int
	TemplatePath   string
	VMStoragePath  string
	OrchestratorIP string
}

// RunnerConfig is the configuration sent to VMs for runner registration
type RunnerConfig struct {
	Token        string `json:"token"`
	Organization string `json:"organization"`
	Repository   string `json:"repository"`
	Name         string `json:"name"`
	Labels       string `json:"labels"`
}

// VMState represents the lifecycle state of a VM
type VMState string

const (
	StateEmpty     VMState = "empty"
	StateCreating  VMState = "creating"
	StateReady     VMState = "ready"
	StateRunning   VMState = "running"
	StateDestroying VMState = "destroying"
)

// VMSlot represents a slot in the VM pool
type VMSlot struct {
	Name        string
	State       VMState
	RunnerToken string
	JobID       int64
	mu          sync.Mutex
}

// ========================================
// VM Manager Interface
// ========================================

// VMManager is the interface for VM operations
// This abstraction allows for platform-specific implementations
type VMManager interface {
	CreateVM(slot *VMSlot) error
	DestroyVM(slot *VMSlot) error
	RunPowerShell(command string) (string, error)
}

// ========================================
// Hyper-V Implementation (Windows)
// ========================================

// HyperVManager implements VMManager for Windows Hyper-V
type HyperVManager struct {
	config Config
	logger *slog.Logger
}

// NewHyperVManager creates a new Hyper-V manager
func NewHyperVManager(config Config, logger *slog.Logger) *HyperVManager {
	return &HyperVManager{
		config: config,
		logger: logger.With("component", "hyperv"),
	}
}

// CreateVM creates a new Hyper-V VM from the template
func (h *HyperVManager) CreateVM(slot *VMSlot) error {
	vmName := slot.Name
	vhdxPath := fmt.Sprintf("%s\\%s.vhdx", h.config.VMStoragePath, vmName)

	// Copy template VHDX to VM storage
	copyCmd := fmt.Sprintf(
		`Copy-Item -Path "%s" -Destination "%s" -Force`,
		h.config.TemplatePath,
		vhdxPath,
	)
	if _, err := h.RunPowerShell(copyCmd); err != nil {
		return fmt.Errorf("failed to copy template: %w", err)
	}

	// Create VM
	createCmd := fmt.Sprintf(`
		New-VM -Name "%s" -MemoryStartupBytes 2GB -Generation 2 -VHDPath "%s"
		Set-VM -Name "%s" -ProcessorCount 2
		Set-VM -Name "%s" -AutomaticStartAction Nothing
		Set-VM -Name "%s" -AutomaticStopAction ShutDown
	`, vmName, vhdxPath, vmName, vmName, vmName)

	if _, err := h.RunPowerShell(createCmd); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	// Start VM
	startCmd := fmt.Sprintf(`Start-VM -Name "%s"`, vmName)
	if _, err := h.RunPowerShell(startCmd); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	h.logger.Info("VM created and started successfully", "vm_name", vmName)
	return nil
}

// DestroyVM destroys a Hyper-V VM and removes its disk
func (h *HyperVManager) DestroyVM(slot *VMSlot) error {
	vmName := slot.Name

	// Stop VM forcefully
	stopCmd := fmt.Sprintf(`Stop-VM -Name "%s" -TurnOff -Force -ErrorAction SilentlyContinue`, vmName)
	_, _ = h.RunPowerShell(stopCmd) // Ignore errors if VM already stopped

	// Remove VM
	removeCmd := fmt.Sprintf(`Remove-VM -Name "%s" -Force`, vmName)
	if _, err := h.RunPowerShell(removeCmd); err != nil {
		return fmt.Errorf("failed to remove VM: %w", err)
	}

	// Delete VHDX file
	vhdxPath := fmt.Sprintf("%s\\%s.vhdx", h.config.VMStoragePath, vmName)
	deleteCmd := fmt.Sprintf(`Remove-Item -Path "%s" -Force -ErrorAction SilentlyContinue`, vhdxPath)
	_, _ = h.RunPowerShell(deleteCmd) // Ignore errors if file already deleted

	h.logger.Info("VM destroyed successfully", "vm_name", vmName)
	return nil
}

// RunPowerShell executes a PowerShell command
func (h *HyperVManager) RunPowerShell(command string) (string, error) {
	cmd := exec.Command("powershell", "-Command", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("powershell error: %w - output: %s", err, string(output))
	}
	return string(output), nil
}

// ========================================
// Mock Implementation (macOS/Testing)
// ========================================

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

	m.simulatedVMs[slot.Name] = "running"
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

// RunPowerShell simulates PowerShell command execution
func (m *MockVMManager) RunPowerShell(command string) (string, error) {
	m.logger.Debug("PowerShell command (simulated)", "command", command)
	return "mock output", nil
}

// ========================================
// Orchestrator
// ========================================

// Orchestrator manages the pool of ephemeral VMs
type Orchestrator struct {
	config    Config
	vmManager VMManager
	vmPool    []*VMSlot
	mu        sync.Mutex
	logger    *slog.Logger
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(config Config, vmManager VMManager, logger *slog.Logger) *Orchestrator {
	return &Orchestrator{
		config:    config,
		vmManager: vmManager,
		vmPool:    make([]*VMSlot, config.PoolSize),
		logger:    logger.With("component", "orchestrator"),
	}
}

// InitializePool creates the initial warm pool of VMs
func (o *Orchestrator) InitializePool() error {
	o.logger.Info("Initializing warm pool of VMs", "pool_size", o.config.PoolSize)

	var wg sync.WaitGroup
	errChan := make(chan error, o.config.PoolSize)

	for i := 0; i < o.config.PoolSize; i++ {
		slotIndex := i
		vmName := fmt.Sprintf("runner-%d", i+1)

		o.vmPool[slotIndex] = &VMSlot{
			Name:  vmName,
			State: StateEmpty,
		}

		wg.Add(1)
		go func(slot *VMSlot) {
			defer wg.Done()
			if err := o.createAndRegisterVM(slot); err != nil {
				errChan <- fmt.Errorf("failed to initialize %s: %w", slot.Name, err)
			}
		}(o.vmPool[slotIndex])
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	o.logger.Info("Warm pool initialized successfully")
	return nil
}

// createAndRegisterVM creates a VM and registers it with GitHub
func (o *Orchestrator) createAndRegisterVM(slot *VMSlot) error {
	slot.mu.Lock()
	slot.State = StateCreating
	slot.mu.Unlock()

	// Generate GitHub runner registration token
	token, err := o.getGitHubRunnerToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	slot.mu.Lock()
	slot.RunnerToken = token
	slot.mu.Unlock()

	// Create the VM
	if err := o.vmManager.CreateVM(slot); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	slot.mu.Lock()
	slot.State = StateReady
	slot.mu.Unlock()

	o.logger.Info("VM ready and waiting for jobs", "vm_name", slot.Name)
	return nil
}

// getGitHubRunnerToken generates a GitHub runner registration token
func (o *Orchestrator) getGitHubRunnerToken() (string, error) {
	// In mock mode, return a fake token without calling GitHub API
	if o.config.GitHubPAT == "mock-token" {
		mockToken := fmt.Sprintf("mock-runner-token-%d", time.Now().UnixNano())
		o.logger.Debug("Generated mock token", "token", mockToken)
		return mockToken, nil
	}

	var url string
	if o.config.GitHubRepo != "" {
		// Repository-level runner
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runners/registration-token",
			o.config.GitHubOrg, o.config.GitHubRepo)
	} else {
		// Organization-level runner
		url = fmt.Sprintf("https://api.github.com/orgs/%s/actions/runners/registration-token",
			o.config.GitHubOrg)
	}

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+o.config.GitHubPAT)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Token, nil
}

// RecreateVM destroys and recreates a VM after job completion
func (o *Orchestrator) RecreateVM(vmName string) error {
	// Find the slot
	var slot *VMSlot
	for _, s := range o.vmPool {
		if s.Name == vmName {
			slot = s
			break
		}
	}

	if slot == nil {
		return fmt.Errorf("VM slot not found: %s", vmName)
	}

	o.logger.Info("Recreating VM", "vm_name", vmName)

	slot.mu.Lock()
	slot.State = StateDestroying
	slot.mu.Unlock()

	// Destroy the VM
	if err := o.vmManager.DestroyVM(slot); err != nil {
		o.logger.Warn("Error destroying VM, continuing with recreation", "vm_name", vmName, "error", err)
		// Continue anyway to try recreation
	}

	// Recreate the VM
	if err := o.createAndRegisterVM(slot); err != nil {
		return fmt.Errorf("failed to recreate VM: %w", err)
	}

	o.logger.Info("VM recreated successfully", "vm_name", vmName)
	return nil
}

// ========================================
// HTTP Handlers
// ========================================

// webhookHandler handles GitHub webhook events
func (o *Orchestrator) webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Verify webhook signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		http.Error(w, "Missing signature", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	if !verifySignature(body, signature, o.config.WebhookSecret) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse webhook payload
	var payload struct {
		Action       string `json:"action"`
		WorkflowJob  struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"workflow_job"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	o.logger.Info("Webhook received",
		"action", payload.Action,
		"job_id", payload.WorkflowJob.ID,
		"status", payload.WorkflowJob.Status)

	// We primarily care about the 'queued' event
	// The VM will pick up the job via GitHub's runner mechanism

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// runnerConfigHandler provides configuration to VMs
func (o *Orchestrator) runnerConfigHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vmName := vars["vmName"]

	// Find the slot
	var slot *VMSlot
	for _, s := range o.vmPool {
		if s.Name == vmName {
			slot = s
			break
		}
	}

	if slot == nil {
		http.Error(w, "VM not found", http.StatusNotFound)
		return
	}

	slot.mu.Lock()
	token := slot.RunnerToken
	state := slot.State
	slot.mu.Unlock()

	if state != StateReady {
		http.Error(w, "VM not ready", http.StatusServiceUnavailable)
		return
	}

	config := RunnerConfig{
		Token:        token,
		Organization: o.config.GitHubOrg,
		Repository:   o.config.GitHubRepo,
		Name:         vmName,
		Labels:       "self-hosted,Windows,X64,ephemeral",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)

	o.logger.Info("Configuration sent to VM", "vm_name", vmName)
}

// runnerCompleteHandler handles job completion notifications from VMs
func (o *Orchestrator) runnerCompleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vmName := vars["vmName"]

	o.logger.Info("Job complete notification received", "vm_name", vmName)

	// Recreate the VM asynchronously
	go func() {
		if err := o.RecreateVM(vmName); err != nil {
			o.logger.Error("Error recreating VM", "vm_name", vmName, "error", err)
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// healthHandler provides health check endpoint
func (o *Orchestrator) healthHandler(w http.ResponseWriter, r *http.Request) {
	status := struct {
		Status  string `json:"status"`
		VMs     int    `json:"vms"`
		Ready   int    `json:"ready"`
		Running int    `json:"running"`
	}{
		Status: "healthy",
		VMs:    len(o.vmPool),
	}

	for _, slot := range o.vmPool {
		slot.mu.Lock()
		state := slot.State
		slot.mu.Unlock()

		switch state {
		case StateReady:
			status.Ready++
		case StateRunning:
			status.Running++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ========================================
// Webhook Signature Verification
// ========================================

// verifySignature verifies the HMAC-SHA256 signature from GitHub
func verifySignature(payload []byte, signature string, secret string) bool {
	if len(signature) < 7 || signature[:7] != "sha256=" {
		return false
	}

	expectedMAC, err := hex.DecodeString(signature[7:])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	actualMAC := mac.Sum(nil)

	return hmac.Equal(actualMAC, expectedMAC)
}

// ========================================
// Main Application
// ========================================

// setupLogger creates and configures the application logger
func setupLogger() *slog.Logger {
	// Get log level from environment (default: INFO)
	logLevel := strings.ToLower(getEnvOrDefault("LOG_LEVEL", "info"))
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Get log format from environment (default: text)
	logFormat := strings.ToLower(getEnvOrDefault("LOG_FORMAT", "text"))

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func main() {
	// Setup logger
	logger := setupLogger()

	// Print version information
	logger.Info("Starting Hyper-V Runner Pool",
		"version", version,
		"commit", commit,
		"built", date)

	// Get current working directory for default paths
	cwd, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get current directory", "error", err)
		os.Exit(1)
	}

	// Default paths relative to repository
	defaultTemplatePath := fmt.Sprintf(`%s\vms\templates\runner-template.vhdx`, cwd)
	defaultStoragePath := fmt.Sprintf(`%s\vms\storage`, cwd)

	// Load configuration from environment
	config := Config{
		GitHubPAT:      os.Getenv("GITHUB_PAT"),
		GitHubOrg:      os.Getenv("GITHUB_ORG"),
		GitHubRepo:     os.Getenv("GITHUB_REPO"),
		WebhookSecret:  os.Getenv("WEBHOOK_SECRET"),
		Port:           getEnvOrDefault("PORT", "8080"),
		PoolSize:       4, // Default pool size
		TemplatePath:   getEnvOrDefault("VM_TEMPLATE_PATH", defaultTemplatePath),
		VMStoragePath:  getEnvOrDefault("VM_STORAGE_PATH", defaultStoragePath),
		OrchestratorIP: getEnvOrDefault("ORCHESTRATOR_IP", "localhost"),
	}

	logger.Info("Using template path", "path", config.TemplatePath)
	logger.Info("Using storage path", "path", config.VMStoragePath)

	// Determine VM manager based on platform
	useMock := os.Getenv("USE_MOCK") == "true"
	var vmManager VMManager

	if useMock {
		logger.Info("Using Mock VM Manager (development mode)")
		vmManager = NewMockVMManager(logger)

		// Use dummy values for mock mode if not provided
		if config.GitHubPAT == "" {
			config.GitHubPAT = "mock-token"
			logger.Debug("Using mock GitHub PAT")
		}
		if config.GitHubOrg == "" {
			config.GitHubOrg = "mock-org"
			logger.Debug("Using mock GitHub organization")
		}
		if config.WebhookSecret == "" {
			config.WebhookSecret = "mock-secret"
			logger.Debug("Using mock webhook secret")
		}
	} else {
		logger.Info("Using Hyper-V VM Manager (production mode)")

		// Validate required configuration for production mode
		if config.GitHubPAT == "" || config.GitHubOrg == "" || config.WebhookSecret == "" {
			logger.Error("Missing required environment variables: GITHUB_PAT, GITHUB_ORG, WEBHOOK_SECRET")
			os.Exit(1)
		}

		vmManager = NewHyperVManager(config, logger)
	}

	// Create orchestrator
	orchestrator := NewOrchestrator(config, vmManager, logger)

	// Initialize VM pool
	if err := orchestrator.InitializePool(); err != nil {
		logger.Error("Failed to initialize pool", "error", err)
		os.Exit(1)
	}

	// Setup HTTP router
	router := mux.NewRouter()
	router.HandleFunc("/webhook", orchestrator.webhookHandler).Methods("POST")
	router.HandleFunc("/api/runner-config/{vmName}", orchestrator.runnerConfigHandler).Methods("GET")
	router.HandleFunc("/api/runner-complete/{vmName}", orchestrator.runnerCompleteHandler).Methods("POST")
	router.HandleFunc("/health", orchestrator.healthHandler).Methods("GET")

	// Start HTTP server
	addr := fmt.Sprintf(":%s", config.Port)
	logger.Info("Orchestrator listening", "port", config.Port)
	if err := http.ListenAndServe(addr, router); err != nil {
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
