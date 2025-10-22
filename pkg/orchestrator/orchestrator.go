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
	if err := o.vmManager.CleanupLeftoverResources(namePrefix); err != nil {
		o.logger.Warn("Cleanup encountered errors (continuing anyway)", "error", err)
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
