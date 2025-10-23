package orchestrator

import (
	"time"

	"hyperv-runner-pool/pkg/vmmanager"
)

// MonitorVMHealth performs comprehensive health monitoring and triggers recreation when unhealthy
func (o *Orchestrator) MonitorVMHealth(slot *vmmanager.VMSlot) {
	healthCheckInterval := time.Duration(o.config.Monitoring.HealthCheckIntervalSeconds) * time.Second
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	o.logger.Debug("Started health monitoring", "vm_name", slot.Name)

	for {
		select {
		case <-o.ctx.Done():
			// Context cancelled, stop monitoring
			o.logger.Debug("Stopping VM monitoring due to shutdown", "vm_name", slot.Name)
			return
		case <-ticker.C:
			if shouldRecreate, reason := o.checkVMHealth(slot); shouldRecreate {
				o.logger.Warn("VM health check failed, recreating",
					"vm_name", slot.Name,
					"reason", reason,
					"state", slot.State,
					"uptime", time.Since(slot.CreatedAt).Round(time.Second),
					"consecutive_failures", slot.HealthCheckFailures+1)

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

// checkVMHealth performs all health checks and returns whether VM should be recreated
// Returns (shouldRecreate bool, reason string)
func (o *Orchestrator) checkVMHealth(slot *vmmanager.VMSlot) (bool, string) {
	now := time.Now()
	creationTimeout := time.Duration(o.config.Monitoring.CreationTimeoutMinutes) * time.Minute
	gracePeriod := time.Duration(o.config.Monitoring.GracePeriodMinutes) * time.Minute

	// 1. Check VM power state
	state, err := o.vmManager.GetVMState(slot.Name)
	if err != nil {
		o.logger.Error("Failed to get VM state", "vm_name", slot.Name, "error", err)
		slot.HealthCheckFailures++
		// Don't recreate on transient API errors, continue monitoring
		return false, ""
	}

	// If VM is stopped/off, it means job completed and VM shut down
	if state == "Off" || state == "Stopped" {
		return true, "VM power state is Off/Stopped"
	}

	// 2. Check if stuck in Creating state
	if slot.State == vmmanager.StateCreating {
		if time.Since(slot.CreatedAt) > creationTimeout {
			return true, "VM stuck in Creating state (timeout)"
		}
		// Still within timeout, don't check GitHub yet
		slot.LastHealthCheck = now
		slot.HealthCheckFailures = 0
		return false, ""
	}

	// 3. Check GitHub runner status (only after grace period)
	timeSinceCreation := time.Since(slot.CreatedAt)
	if timeSinceCreation > gracePeriod {
		runner, err := o.githubClient.GetRunnerByName(slot.Name)
		if err != nil {
			o.logger.Error("Failed to check runner status in GitHub",
				"vm_name", slot.Name,
				"error", err)
			slot.HealthCheckFailures++
			// Don't recreate on transient GitHub API errors
			return false, ""
		}

		// Runner not found in GitHub
		if runner == nil {
			return true, "Runner not found in GitHub after grace period"
		}

		// Runner is offline
		if runner.Status != "online" {
			return true, "Runner is offline in GitHub"
		}

		// Log successful health check at debug level
		o.logger.Debug("Health check passed",
			"vm_name", slot.Name,
			"github_status", runner.Status,
			"uptime", timeSinceCreation.Round(time.Second))
	}

	// All checks passed
	slot.LastHealthCheck = now
	slot.HealthCheckFailures = 0
	return false, ""
}
