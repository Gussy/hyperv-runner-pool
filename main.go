package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
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
	GitHub  GitHubConfig  `yaml:"github"`
	Runners RunnersConfig `yaml:"runners"`
	HyperV  HyperVConfig  `yaml:"hyperv"`
	Debug   DebugConfig   `yaml:"debug"`
}

// GitHubConfig holds GitHub-specific configuration
type GitHubConfig struct {
	Token string `yaml:"token"`
	Org   string `yaml:"org"`
	Repo  string `yaml:"repo"`
}

// RunnersConfig holds runner pool configuration
type RunnersConfig struct {
	PoolSize   int    `yaml:"pool_size"`
	NamePrefix string `yaml:"name_prefix"`
}

// HyperVConfig holds Hyper-V specific configuration
type HyperVConfig struct {
	TemplatePath  string `yaml:"template_path"`
	VMStoragePath string `yaml:"storage_path"`
}

// DebugConfig holds debugging and logging configuration
type DebugConfig struct {
	UseMock   bool   `yaml:"use_mock"`
	LogLevel  string `yaml:"log_level"`
	LogFormat string `yaml:"log_format"`
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
	GetVMState(vmName string) (string, error)
	InjectConfig(vhdxPath string, config RunnerConfig) error
	RunPowerShell(command string) (string, error)
	CleanupLeftoverResources(namePrefix string) error
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
	vhdxPath := fmt.Sprintf("%s\\%s.vhdx", h.config.HyperV.VMStoragePath, vmName)

	// Copy template VHDX to VM storage
	copyCmd := fmt.Sprintf(
		`Copy-Item -Path "%s" -Destination "%s" -Force`,
		h.config.HyperV.TemplatePath,
		vhdxPath,
	)
	if _, err := h.RunPowerShell(copyCmd); err != nil {
		return fmt.Errorf("failed to copy template: %w", err)
	}

	// Inject runner config into VHDX (before creating VM)
	runnerConfig := RunnerConfig{
		Token:        slot.RunnerToken,
		Organization: h.config.GitHub.Org,
		Repository:   h.config.GitHub.Repo,
		Name:         vmName,
		Labels:       "self-hosted,Windows,X64,ephemeral",
	}

	if err := h.InjectConfig(vhdxPath, runnerConfig); err != nil {
		return fmt.Errorf("failed to inject config: %w", err)
	}

	// Create VM
	createCmd := fmt.Sprintf(`
		New-VM -Name "%s" -MemoryStartupBytes 2GB -Generation 2 -VHDPath "%s"
		Set-VM -Name "%s" -ProcessorCount 2
		Set-VM -Name "%s" -AutomaticStartAction Nothing
		Set-VM -Name "%s" -AutomaticStopAction ShutDown
		Add-VMNetworkAdapter -VMName "%s" -SwitchName "Default Switch"
		$vmDrive = Get-VMHardDiskDrive -VMName "%s"
		Set-VMFirmware -VMName "%s" -BootOrder $vmDrive
	`, vmName, vhdxPath, vmName, vmName, vmName, vmName, vmName, vmName)

	if _, err := h.RunPowerShell(createCmd); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	// Start VM
	startCmd := fmt.Sprintf(`Start-VM -Name "%s"`, vmName)
	if _, err := h.RunPowerShell(startCmd); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	h.logger.Info("VM created and started successfully", "vm_name", vmName)
	h.logger.Info("Scheduled task should start automatically on boot", "vm_name", vmName)

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
	vhdxPath := fmt.Sprintf("%s\\%s.vhdx", h.config.HyperV.VMStoragePath, vmName)
	deleteCmd := fmt.Sprintf(`Remove-Item -Path "%s" -Force -ErrorAction SilentlyContinue`, vhdxPath)
	_, _ = h.RunPowerShell(deleteCmd) // Ignore errors if file already deleted

	h.logger.Info("VM destroyed successfully", "vm_name", vmName)
	return nil
}

// GetVMState returns the current state of a VM (Running, Off, Stopped, etc.)
func (h *HyperVManager) GetVMState(vmName string) (string, error) {
	cmd := fmt.Sprintf(`(Get-VM -Name "%s").State`, vmName)
	output, err := h.RunPowerShell(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get VM state: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// InjectConfig mounts the VHDX, writes runner config, then unmounts
func (h *HyperVManager) InjectConfig(vhdxPath string, config RunnerConfig) error {
	h.logger.Debug("Starting config injection", "vhdx_path", vhdxPath)

	// Mount the VHDX with detailed partition information
	mountCmd := fmt.Sprintf(`
		$ErrorActionPreference = "Stop"
		$disk = Mount-VHD -Path "%s" -Passthru
		$diskNumber = $disk.Number
		Write-Output "DiskNumber: $diskNumber"

		# Get all partitions to see what's available
		$partitions = Get-Partition -DiskNumber $diskNumber
		Write-Output "Partitions found: $($partitions.Count)"
		$partitions | ForEach-Object {
			Write-Output "  Partition $($_.PartitionNumber): Type=$($_.Type), Size=$($_.Size), DriveLetter=$($_.DriveLetter)"
		}

		# Try to find the main Windows partition
		# It should be the largest Basic partition, or the one with a drive letter
		$partition = $partitions | Where-Object { $_.Type -eq 'Basic' -and $_.DriveLetter } | Select-Object -First 1

		if (-not $partition) {
			# If no partition with drive letter, try to assign one to the largest Basic partition
			$partition = $partitions | Where-Object { $_.Type -eq 'Basic' } | Sort-Object Size -Descending | Select-Object -First 1
			if ($partition -and -not $partition.DriveLetter) {
				Write-Output "Assigning drive letter to partition $($partition.PartitionNumber)..."
				$partition | Add-PartitionAccessPath -AssignDriveLetter
				$partition = Get-Partition -DiskNumber $diskNumber -PartitionNumber $partition.PartitionNumber
			}
		}

		if (-not $partition) {
			throw "No suitable partition found on disk"
		}

		$driveLetter = $partition.DriveLetter
		if (-not $driveLetter) {
			throw "Failed to get drive letter for partition"
		}

		Write-Output "DRIVE_LETTER:$driveLetter"
	`, vhdxPath)

	output, err := h.RunPowerShell(mountCmd)
	if err != nil {
		return fmt.Errorf("failed to mount VHDX: %w", err)
	}

	h.logger.Debug("Mount output", "output", output)

	// Parse drive letter from output
	var driveLetter string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "DRIVE_LETTER:") {
			driveLetter = strings.TrimPrefix(line, "DRIVE_LETTER:")
			driveLetter = strings.TrimSpace(driveLetter)
			break
		}
	}

	if driveLetter == "" {
		return fmt.Errorf("failed to extract drive letter from mount output: %s", output)
	}

	h.logger.Info("VHDX mounted successfully", "drive_letter", driveLetter)

	// Ensure we unmount on exit
	defer func() {
		unmountCmd := fmt.Sprintf(`Dismount-VHD -Path "%s"`, vhdxPath)
		if _, err := h.RunPowerShell(unmountCmd); err != nil {
			h.logger.Warn("Failed to unmount VHDX", "path", vhdxPath, "error", err)
		} else {
			h.logger.Debug("VHDX unmounted successfully", "path", vhdxPath)
		}
	}()

	// Write config file
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	h.logger.Debug("Config JSON created", "size_bytes", len(configJSON))

	// Write JSON to a temporary file first to avoid command line length/escaping issues
	tempFile := fmt.Sprintf("%s\\runner-config-temp.json", os.TempDir())
	if err := os.WriteFile(tempFile, configJSON, 0644); err != nil {
		return fmt.Errorf("failed to write temp config file: %w", err)
	}
	defer os.Remove(tempFile) // Clean up temp file

	h.logger.Debug("Temp config file created", "path", tempFile)

	// Copy the temp file to the mounted VHDX and verify
	destPath := fmt.Sprintf("%s:\\runner-config.json", driveLetter)
	copyAndVerifyCmd := fmt.Sprintf(`
		$ErrorActionPreference = "Stop"
		$source = "%s"
		$dest = "%s"

		Write-Output "Copying from: $source"
		Write-Output "Copying to: $dest"

		if (-not (Test-Path $source)) {
			throw "Source file not found: $source"
		}

		Copy-Item -Path $source -Destination $dest -Force

		if (-not (Test-Path $dest)) {
			throw "Copy failed - destination file not found: $dest"
		}

		$copiedSize = (Get-Item $dest).Length
		Write-Output "File copied successfully. Size: $copiedSize bytes"

		# Verify content
		$content = Get-Content $dest -Raw
		Write-Output "Content preview: $($content.Substring(0, [Math]::Min(100, $content.Length)))..."

		Write-Output "SUCCESS"
	`, tempFile, destPath)

	copyOutput, err := h.RunPowerShell(copyAndVerifyCmd)
	if err != nil {
		return fmt.Errorf("failed to copy config to VHDX: %w", err)
	}

	h.logger.Debug("Copy operation output", "output", copyOutput)

	if !strings.Contains(copyOutput, "SUCCESS") {
		return fmt.Errorf("config copy verification failed: %s", copyOutput)
	}

	h.logger.Info("Config injected and verified successfully",
		"vhdx_path", vhdxPath,
		"destination", destPath,
		"config_size", len(configJSON))

	return nil
}

// CleanupLeftoverResources removes any VMs and VHDXs matching the name prefix from previous runs
func (h *HyperVManager) CleanupLeftoverResources(namePrefix string) error {
	h.logger.Info("Cleaning up leftover resources from previous runs", "name_prefix", namePrefix)

	cleanupCmd := fmt.Sprintf(`
		$ErrorActionPreference = "Continue"
		$namePrefix = "%s"
		$storagePath = "%s"
		$cleaned = 0

		# Find and remove VMs matching the prefix
		$vms = Get-VM | Where-Object { $_.Name -like "$namePrefix*" }
		foreach ($vm in $vms) {
			Write-Output "Removing VM: $($vm.Name)"
			try {
				Stop-VM -Name $vm.Name -TurnOff -Force -ErrorAction SilentlyContinue
				Remove-VM -Name $vm.Name -Force -ErrorAction Stop
				$cleaned++
				Write-Output "  Removed successfully"
			} catch {
				Write-Output "  Warning: Failed to remove VM: $_"
			}
		}

		# Find and remove orphaned VHDX files matching the prefix
		if (Test-Path $storagePath) {
			$vhdxFiles = Get-ChildItem -Path $storagePath -Filter "$namePrefix*.vhdx" -ErrorAction SilentlyContinue
			foreach ($file in $vhdxFiles) {
				Write-Output "Removing VHDX: $($file.Name)"
				try {
					# Try to dismount if mounted
					Dismount-VHD -Path $file.FullName -ErrorAction SilentlyContinue

					# Delete the file
					Remove-Item -Path $file.FullName -Force -ErrorAction Stop
					$cleaned++
					Write-Output "  Removed successfully"
				} catch {
					Write-Output "  Warning: Failed to remove VHDX: $_"
				}
			}
		}

		Write-Output "Cleanup complete. Removed $cleaned resources."
		if ($cleaned -gt 0) {
			Write-Output "CLEANUP_PERFORMED"
		}
	`, namePrefix, h.config.HyperV.VMStoragePath)

	output, err := h.RunPowerShell(cleanupCmd)
	if err != nil {
		h.logger.Warn("Cleanup encountered errors (this is usually okay)", "error", err, "output", output)
		// Don't fail startup due to cleanup errors - they're often expected
	} else {
		h.logger.Debug("Cleanup output", "output", output)
		if strings.Contains(output, "CLEANUP_PERFORMED") {
			h.logger.Info("Leftover resources cleaned up successfully")
		} else {
			h.logger.Debug("No leftover resources found")
		}
	}

	return nil
}

// RunPowerShell executes a PowerShell command by writing it to a temp file and executing it
// This approach is more robust than -Command for multi-line scripts and avoids escaping issues
func (h *HyperVManager) RunPowerShell(command string) (string, error) {
	// Create a temporary PowerShell script file
	tempFile, err := os.CreateTemp("", "hyperv-runner-*.ps1")
	if err != nil {
		return "", fmt.Errorf("failed to create temp script file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write the command to the temp file
	if _, err := tempFile.WriteString(command); err != nil {
		tempFile.Close()
		return "", fmt.Errorf("failed to write to temp script file: %w", err)
	}
	tempFile.Close()

	// Log the command at debug level (truncate if very long)
	commandPreview := command
	if len(commandPreview) > 200 {
		commandPreview = commandPreview[:200] + "... (truncated)"
	}
	h.logger.Debug("Executing PowerShell script",
		"script_file", tempFile.Name(),
		"command_preview", commandPreview,
		"command_length", len(command))

	// Optionally save to debug directory for manual testing
	if debugDir := os.Getenv("POWERSHELL_DEBUG_DIR"); debugDir != "" {
		timestamp := time.Now().Format("20060102-150405.000")
		debugFile := fmt.Sprintf("%s\\ps-%s.ps1", debugDir, timestamp)
		if err := os.WriteFile(debugFile, []byte(command), 0644); err != nil {
			h.logger.Warn("Failed to save debug script", "path", debugFile, "error", err)
		} else {
			h.logger.Debug("Saved PowerShell script to debug directory", "path", debugFile)
		}
	}

	// Execute PowerShell with -File parameter (more robust than -Command)
	cmd := exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-NoProfile", "-File", tempFile.Name())

	// Capture stdout and stderr separately for better debugging
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	if err != nil {
		// Build detailed error message
		errMsg := fmt.Sprintf("powershell error: %v", err)
		if len(stdoutStr) > 0 {
			errMsg += fmt.Sprintf("\nstdout: %s", stdoutStr)
		}
		if len(stderrStr) > 0 {
			errMsg += fmt.Sprintf("\nstderr: %s", stderrStr)
		}
		errMsg += fmt.Sprintf("\nscript_file: %s (saved for debugging)", tempFile.Name())
		errMsg += fmt.Sprintf("\ncommand_preview: %s", commandPreview)

		return stdoutStr + stderrStr, fmt.Errorf("%s", errMsg)
	}

	// Return combined output (stdout + stderr)
	output := stdoutStr
	if len(stderrStr) > 0 {
		output += stderrStr
	}

	h.logger.Debug("PowerShell script executed successfully",
		"output_length", len(output))

	return output, nil
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
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(config Config, vmManager VMManager, logger *slog.Logger) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Orchestrator{
		config:    config,
		vmManager: vmManager,
		vmPool:    make([]*VMSlot, config.Runners.PoolSize),
		logger:    logger.With("component", "orchestrator"),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// InitializePool creates the initial warm pool of VMs
func (o *Orchestrator) InitializePool() error {
	// First, cleanup any leftover resources from previous runs
	namePrefix := o.config.Runners.NamePrefix
	if namePrefix == "" {
		namePrefix = "runner-"
	}

	o.logger.Info("Performing startup cleanup", "name_prefix", namePrefix)
	if err := o.vmManager.CleanupLeftoverResources(namePrefix); err != nil {
		o.logger.Warn("Cleanup encountered errors (continuing anyway)", "error", err)
	}

	o.logger.Info("Initializing warm pool of VMs", "pool_size", o.config.Runners.PoolSize)

	var wg sync.WaitGroup
	errChan := make(chan error, o.config.Runners.PoolSize)

	for i := 0; i < o.config.Runners.PoolSize; i++ {
		slotIndex := i
		vmName := fmt.Sprintf("%s%d", namePrefix, i+1)

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

	// Create the VM (config is injected during creation)
	if err := o.vmManager.CreateVM(slot); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	slot.mu.Lock()
	slot.State = StateReady
	slot.mu.Unlock()

	// Start monitoring VM state in background
	go o.MonitorVMState(slot)

	o.logger.Info("VM ready and waiting for jobs", "vm_name", slot.Name)
	return nil
}

// getGitHubRunnerToken generates a GitHub runner registration token
func (o *Orchestrator) getGitHubRunnerToken() (string, error) {
	// In mock mode, return a fake token without calling GitHub API
	if o.config.GitHub.Token == "mock-token" {
		mockToken := fmt.Sprintf("mock-runner-token-%d", time.Now().UnixNano())
		o.logger.Debug("Generated mock token", "token", mockToken)
		return mockToken, nil
	}

	var url string
	if o.config.GitHub.Repo != "" {
		// Repository-level runner
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runners/registration-token",
			o.config.GitHub.Org, o.config.GitHub.Repo)
	} else {
		// Organization-level runner
		url = fmt.Sprintf("https://api.github.com/orgs/%s/actions/runners/registration-token",
			o.config.GitHub.Org)
	}

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+o.config.GitHub.Token)
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
// VM State Monitoring
// ========================================

// MonitorVMState polls VM state and triggers recreation when VM stops
func (o *Orchestrator) MonitorVMState(slot *VMSlot) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			// Context cancelled, stop monitoring
			o.logger.Debug("Stopping VM monitoring due to shutdown", "vm_name", slot.Name)
			return
		case <-ticker.C:
			state, err := o.vmManager.GetVMState(slot.Name)
			if err != nil {
				o.logger.Error("Failed to get VM state", "vm_name", slot.Name, "error", err)
				continue
			}

			// If VM is stopped/off, it means job completed and VM shut down
			if state == "Off" || state == "Stopped" {
				o.logger.Info("VM stopped, recreating", "vm_name", slot.Name)
				ticker.Stop()

				// Recreate the VM asynchronously
				go func() {
					if err := o.RecreateVM(slot.Name); err != nil {
						o.logger.Error("Error recreating VM", "vm_name", slot.Name, "error", err)
					}
				}()
				return
			}
		}
	}
}

// Shutdown gracefully shuts down the orchestrator and cleans up all VMs
func (o *Orchestrator) Shutdown() error {
	o.logger.Info("Shutting down orchestrator and cleaning up VMs...")

	// Cancel context to stop all monitoring goroutines
	o.cancel()

	// Give monitoring goroutines a moment to stop
	time.Sleep(1 * time.Second)

	namePrefix := o.config.Runners.NamePrefix
	if namePrefix == "" {
		namePrefix = "runner-"
	}

	// Cleanup all VMs
	if err := o.vmManager.CleanupLeftoverResources(namePrefix); err != nil {
		o.logger.Warn("Errors during shutdown cleanup", "error", err)
		return err
	}

	o.logger.Info("Orchestrator shutdown complete")
	return nil
}

// ========================================
// Configuration Loading
// ========================================

// loadConfigFromFile loads configuration from a YAML file
func loadConfigFromFile(path string) (*Config, error) {
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
	if config.Debug.LogLevel == "" {
		config.Debug.LogLevel = "info"
	}
	if config.Debug.LogFormat == "" {
		config.Debug.LogFormat = "text"
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
		if config.GitHub.Token == "" || config.GitHub.Org == "" {
			return nil, fmt.Errorf("github.token and github.org are required when debug.use_mock is false")
		}
	} else {
		// Set dummy values for mock mode if not provided
		if config.GitHub.Token == "" {
			config.GitHub.Token = "mock-token"
		}
		if config.GitHub.Org == "" {
			config.GitHub.Org = "mock-org"
		}
	}

	return &config, nil
}

// ========================================
// Main Application
// ========================================

// setupLogger creates and configures the application logger
func setupLogger(logLevel, logFormat string) *slog.Logger {
	// Parse log level
	logLevel = strings.ToLower(logLevel)
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

	// Parse log format
	logFormat = strings.ToLower(logFormat)

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
	app := &cli.Command{
		Name:    "hyperv-runner-pool",
		Usage:   "Manage a pool of ephemeral Hyper-V VMs for GitHub Actions runners",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Usage:    "Path to YAML configuration file",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			configPath := cmd.String("config")

			// Load configuration from YAML file
			config, err := loadConfigFromFile(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Setup logger with config
			logger := setupLogger(config.Debug.LogLevel, config.Debug.LogFormat)

			// Print version information
			logger.Info("Starting Hyper-V Runner Pool",
				"version", version,
				"commit", commit,
				"built", date)

			logger.Info("Configuration loaded",
				"config_file", configPath,
				"pool_size", config.Runners.PoolSize,
				"mock_mode", config.Debug.UseMock)
			logger.Info("Using template path", "path", config.HyperV.TemplatePath)
			logger.Info("Using storage path", "path", config.HyperV.VMStoragePath)

			// Determine VM manager based on config
			var vmManager VMManager

			if config.Debug.UseMock {
				logger.Info("Using Mock VM Manager (development mode)")
				vmManager = NewMockVMManager(logger)
			} else {
				logger.Info("Using Hyper-V VM Manager (production mode)")
				vmManager = NewHyperVManager(*config, logger)
			}

			// Create orchestrator
			orchestrator := NewOrchestrator(*config, vmManager, logger)

			// Initialize VM pool
			if err := orchestrator.InitializePool(); err != nil {
				return fmt.Errorf("failed to initialize pool: %w", err)
			}

			// Setup signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			// Keep the orchestrator running
			logger.Info("Orchestrator running, monitoring VMs for job completion")
			logger.Info("Press Ctrl+C to shutdown gracefully")

			// Wait for shutdown signal
			sig := <-sigChan
			logger.Info("Received shutdown signal", "signal", sig.String())

			// Perform graceful shutdown
			if err := orchestrator.Shutdown(); err != nil {
				logger.Error("Error during shutdown", "error", err)
				return err
			}

			logger.Info("Shutdown complete")
			return nil
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
