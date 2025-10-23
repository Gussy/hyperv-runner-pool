package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"hyperv-runner-pool/pkg/config"
	"hyperv-runner-pool/pkg/github"
	"hyperv-runner-pool/pkg/vmmanager"
)

// Orchestrator manages the pool of ephemeral VMs
type Orchestrator struct {
	config       config.Config
	vmManager    vmmanager.VMManager
	githubClient *github.Client
	vmPool       []*vmmanager.VMSlot
	mu           sync.Mutex
	logger       *slog.Logger
	ctx          context.Context
	cancel       context.CancelFunc
}

// New creates a new orchestrator instance
func New(cfg config.Config, vmMgr vmmanager.VMManager, ghClient *github.Client, logger *slog.Logger) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Orchestrator{
		config:       cfg,
		vmManager:    vmMgr,
		githubClient: ghClient,
		vmPool:       make([]*vmmanager.VMSlot, cfg.Runners.PoolSize),
		logger:       logger.With("component", "orchestrator"),
		ctx:          ctx,
		cancel:       cancel,
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

	// Cleanup VMs and VHDXs
	if err := o.vmManager.CleanupLeftoverResources(namePrefix); err != nil {
		o.logger.Warn("VM cleanup encountered errors (continuing anyway)", "error", err)
	}

	// Cleanup offline runners from GitHub
	if err := o.cleanupOfflineRunners(namePrefix); err != nil {
		o.logger.Warn("GitHub runner cleanup encountered errors (continuing anyway)", "error", err)
	}

	o.logger.Info("Initializing warm pool of VMs", "pool_size", o.config.Runners.PoolSize)

	var wg sync.WaitGroup
	errChan := make(chan error, o.config.Runners.PoolSize)

	for i := 0; i < o.config.Runners.PoolSize; i++ {
		slotIndex := i
		vmName := fmt.Sprintf("%s%d", namePrefix, i+1)

		o.vmPool[slotIndex] = &vmmanager.VMSlot{
			Name:  vmName,
			State: vmmanager.StateEmpty,
		}

		wg.Add(1)
		go func(slot *vmmanager.VMSlot) {
			defer wg.Done()
			if err := o.createAndRegisterVM(slot); err != nil {
				errChan <- fmt.Errorf("failed to initialize %s: %w", slot.Name, err)
			}
		}(o.vmPool[slotIndex])
	}

	wg.Wait()
	close(errChan)

	// Collect all errors
	var errors []error
	for err := range errChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	// Return combined error if any occurred
	if len(errors) > 0 {
		for _, err := range errors {
			o.logger.Error("VM initialization failed", "error", err)
		}
		return fmt.Errorf("failed to initialize %d VMs: %v", len(errors), errors)
	}

	o.logger.Info("Warm pool initialized successfully")
	return nil
}

// createAndRegisterVM creates a VM and registers it with GitHub
func (o *Orchestrator) createAndRegisterVM(slot *vmmanager.VMSlot) error {
	slot.State = vmmanager.StateCreating

	// Generate GitHub runner registration token
	token, err := o.githubClient.GetRunnerToken()
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	slot.RunnerToken = token

	// Create the VM (config is injected during creation)
	if err := o.vmManager.CreateVM(slot); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	slot.State = vmmanager.StateReady

	// Start monitoring VM state in background
	go o.MonitorVMState(slot)

	o.logger.Info("VM ready and waiting for jobs", "vm_name", slot.Name)
	return nil
}

// RecreateVM destroys and recreates a VM after job completion
func (o *Orchestrator) RecreateVM(vmName string) error {
	// Find the slot
	var slot *vmmanager.VMSlot
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

	slot.State = vmmanager.StateDestroying

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

// RestartAllVMs restarts all VMs in the pool
func (o *Orchestrator) RestartAllVMs() error {
	o.logger.Info("Restarting all VMs in pool", "pool_size", len(o.vmPool))

	o.mu.Lock()
	defer o.mu.Unlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(o.vmPool))

	for _, slot := range o.vmPool {
		if slot == nil {
			continue
		}

		wg.Add(1)
		go func(s *vmmanager.VMSlot) {
			defer wg.Done()

			o.logger.Info("Restarting VM", "vm_name", s.Name)

			s.State = vmmanager.StateDestroying

			// Destroy the VM
			if err := o.vmManager.DestroyVM(s); err != nil {
				o.logger.Warn("Error destroying VM during restart", "vm_name", s.Name, "error", err)
				// Continue anyway to try recreation
			}

			// Recreate the VM
			if err := o.createAndRegisterVM(s); err != nil {
				errChan <- fmt.Errorf("failed to restart %s: %w", s.Name, err)
				return
			}

			o.logger.Info("VM restarted successfully", "vm_name", s.Name)
		}(slot)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			o.logger.Error("VM restart failed", "error", err)
		}
		return fmt.Errorf("failed to restart %d VMs: %v", len(errors), errors)
	}

	o.logger.Info("All VMs restarted successfully")
	return nil
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

	// Cleanup offline runners from GitHub first (before destroying VMs)
	// This ensures we remove any stale offline runners
	if err := o.cleanupOfflineRunners(namePrefix); err != nil {
		o.logger.Warn("GitHub runner cleanup encountered errors during shutdown (continuing)", "error", err)
		// Don't fail shutdown due to GitHub API errors
	}

	// Cleanup all VMs
	if err := o.vmManager.CleanupLeftoverResources(namePrefix); err != nil {
		o.logger.Warn("Errors during shutdown cleanup", "error", err)
		return err
	}

	o.logger.Info("Orchestrator shutdown complete")
	return nil
}

// cleanupOfflineRunners removes all runners from GitHub that match the name prefix
// Note: Despite the function name, this removes runners regardless of online/offline status
// to handle cases where the program is restarted quickly before runners appear offline
func (o *Orchestrator) cleanupOfflineRunners(namePrefix string) error {
	o.logger.Info("Checking for runners to cleanup in GitHub", "name_prefix", namePrefix)

	// List all runners from GitHub
	runners, err := o.githubClient.ListRunners()
	if err != nil {
		return fmt.Errorf("failed to list runners: %w", err)
	}

	if len(runners) == 0 {
		o.logger.Debug("No runners found in GitHub")
		return nil
	}

	o.logger.Debug("Found runners in GitHub", "count", len(runners))

	// Log all runners for debugging
	for _, runner := range runners {
		o.logger.Debug("GitHub runner details", "name", runner.Name, "id", runner.ID, "status", runner.Status)
	}

	// Find offline runners matching our name prefix
	var toRemove []github.RunnerInfo
	for _, runner := range runners {
		// Check if runner name matches our prefix pattern (e.g., "windows-latest-1", "windows-latest-2")
		// Use simple prefix match + digit check
		if len(runner.Name) > len(namePrefix) &&
			runner.Name[:len(namePrefix)] == namePrefix {

			suffix := runner.Name[len(namePrefix):]

			// Verify the suffix is digits (to match our pool naming pattern)
			isDigitsOnly := true
			for _, ch := range suffix {
				if ch < '0' || ch > '9' {
					isDigitsOnly = false
					break
				}
			}

			if !isDigitsOnly {
				o.logger.Debug("Skipping runner - suffix not digits", "name", runner.Name, "suffix", suffix)
				continue
			}

			// Remove all matching runners regardless of status
			// (they may still show as "online" if we restarted quickly)
			o.logger.Debug("Marking runner for removal", "name", runner.Name, "status", runner.Status)
			toRemove = append(toRemove, runner)
		}
	}

	if len(toRemove) == 0 {
		o.logger.Info("No matching runners to remove")
		return nil
	}

	o.logger.Info("Removing matching runners from GitHub", "count", len(toRemove))

	// Remove each runner
	var errors []error
	for _, runner := range toRemove {
		o.logger.Info("Removing runner", "runner_name", runner.Name, "runner_id", runner.ID, "status", runner.Status)
		if err := o.githubClient.RemoveRunner(runner.ID, runner.Name); err != nil {
			o.logger.Warn("Failed to remove runner", "runner_name", runner.Name, "error", err)
			errors = append(errors, fmt.Errorf("failed to remove %s: %w", runner.Name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors removing runners", len(errors))
	}

	o.logger.Info("Successfully removed runners from GitHub", "count", len(toRemove))
	return nil
}
