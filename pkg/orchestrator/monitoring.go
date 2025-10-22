package orchestrator

import (
	"time"

	"hyperv-runner-pool/pkg/vmmanager"
)

// MonitorVMState polls VM state and triggers recreation when VM stops
func (o *Orchestrator) MonitorVMState(slot *vmmanager.VMSlot) {
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
