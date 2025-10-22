package vmmanager

import (
	"sync"
)

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

// RunnerConfig is the configuration sent to VMs for runner registration
type RunnerConfig struct {
	Token        string `json:"token"`
	Organization string `json:"organization"`
	Repository   string `json:"repository"`
	Name         string `json:"name"`
	Labels       string `json:"labels"`
	RunnerGroup  string `json:"runner_group,omitempty"` // Optional: for org-level runners only
}

// VMState represents the lifecycle state of a VM
type VMState string

const (
	StateEmpty      VMState = "empty"
	StateCreating   VMState = "creating"
	StateReady      VMState = "ready"
	StateRunning    VMState = "running"
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
