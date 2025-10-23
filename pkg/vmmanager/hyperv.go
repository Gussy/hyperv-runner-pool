package vmmanager

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"hyperv-runner-pool/pkg/config"
)

//go:embed scripts/configure-runner.ps1
var configureRunnerScript string

// HyperVManager implements VMManager for Windows Hyper-V
type HyperVManager struct {
	config config.Config
	logger *slog.Logger
}

// NewHyperVManager creates a new Hyper-V manager
func NewHyperVManager(cfg config.Config, logger *slog.Logger) *HyperVManager {
	return &HyperVManager{
		config: cfg,
		logger: logger.With("component", "hyperv"),
	}
}

// CreateVM creates a new Hyper-V VM from the template
func (h *HyperVManager) CreateVM(slot *VMSlot) error {
	vmName := slot.Name
	vhdxPath := fmt.Sprintf("%s\\%s.vhdx", h.config.HyperV.VMStoragePath, vmName)

	h.logger.Info("Starting VM creation", "vm_name", vmName)

	// Create differencing disk (child VHDX) referencing the parent template
	// This is much faster than copying the entire VHDX (~1s vs 15s) and uses less storage
	// The child disk only stores changes from the parent template
	// NOTE: Parent template must be read-only to prevent corruption of child disks
	//       Run: Set-ItemProperty -Path "template.vhdx" -Name IsReadOnly -Value $true
	h.logger.Debug("Creating differencing disk", "vm_name", vmName)
	createDiffCmd := fmt.Sprintf(
		`New-VHD -ParentPath "%s" -Path "%s" -Differencing`,
		h.config.HyperV.TemplatePath,
		vhdxPath,
	)
	if _, err := h.RunPowerShell(createDiffCmd); err != nil {
		return fmt.Errorf("failed to create differencing disk: %w", err)
	}
	h.logger.Debug("Differencing disk created", "vm_name", vmName)

	// Inject runner config into VHDX (before creating VM)
	// Build labels: start with defaults, then add custom labels
	defaultLabels := []string{"self-hosted", "Windows", "X64", "ephemeral"}
	allLabels := append(defaultLabels, h.config.Runners.Labels...)
	labelsStr := strings.Join(allLabels, ",")

	runnerConfig := RunnerConfig{
		Token:        slot.RunnerToken,
		Organization: h.config.GitHub.GetAccount(),
		Repository:   h.config.GitHub.Repo,
		Name:         vmName,
		Labels:       labelsStr,
		RunnerGroup:  h.config.Runners.RunnerGroup,
	}

	// Add cache URL if configured
	if h.config.Runners.CacheURL != "" {
		runnerConfig.CacheURL = h.config.Runners.CacheURL
		h.logger.Debug("Cache URL configured", "cache_url", h.config.Runners.CacheURL)
	}

	h.logger.Debug("Injecting runner config", "vm_name", vmName)
	if err := h.InjectConfig(vhdxPath, runnerConfig); err != nil {
		return fmt.Errorf("failed to inject config: %w", err)
	}
	h.logger.Debug("Runner config injected", "vm_name", vmName)

	// Create VM
	h.logger.Debug("Creating VM in Hyper-V", "vm_name", vmName, "memory_mb", h.config.HyperV.VMMemoryMB, "cpu_count", h.config.HyperV.VMCPUCount)
	createCmd := fmt.Sprintf(`
		New-VM -Name "%s" -MemoryStartupBytes %dMB -Generation 2 -VHDPath "%s"
		Set-VM -Name "%s" -ProcessorCount %d
		Set-VM -Name "%s" -AutomaticStartAction Nothing
		Set-VM -Name "%s" -AutomaticStopAction ShutDown
		Add-VMNetworkAdapter -VMName "%s" -SwitchName "Default Switch"
		$vmDrive = Get-VMHardDiskDrive -VMName "%s"
		Set-VMFirmware -VMName "%s" -BootOrder $vmDrive
	`, vmName, h.config.HyperV.VMMemoryMB, vhdxPath, vmName, h.config.HyperV.VMCPUCount, vmName, vmName, vmName, vmName, vmName)

	if _, err := h.RunPowerShell(createCmd); err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}
	h.logger.Debug("VM created in Hyper-V", "vm_name", vmName)

	// Start VM
	h.logger.Debug("Starting VM", "vm_name", vmName)
	startCmd := fmt.Sprintf(`Start-VM -Name "%s"`, vmName)
	if _, err := h.RunPowerShell(startCmd); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	h.logger.Info("VM created and started successfully", "vm_name", vmName)
	h.logger.Info("Waiting for VM to boot and configuring runner...", "vm_name", vmName)

	// Execute the embedded configure-runner script in the VM
	// This will set up the scheduled task and start the runner
	h.logger.Debug("Executing configure script in VM", "vm_name", vmName)
	if err := h.ExecuteScriptInVM(vmName, configureRunnerScript); err != nil {
		return fmt.Errorf("failed to configure runner in VM: %w", err)
	}

	h.logger.Info("Runner configured successfully in VM", "vm_name", vmName)

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
	// Use VM-specific name to avoid race conditions when creating multiple VMs in parallel
	tempFile := fmt.Sprintf("%s\\runner-config-%s.json", os.TempDir(), config.Name)
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

// ExecuteScriptInVM executes a PowerShell script inside a running VM using PowerShell Direct
// This method uses stored credentials to avoid interactive prompts
func (h *HyperVManager) ExecuteScriptInVM(vmName string, scriptContent string) error {
	h.logger.Info("Executing script in VM via PowerShell Direct", "vm_name", vmName)

	// Escape single quotes and backticks in the script content
	escapedScript := strings.ReplaceAll(scriptContent, "'", "''")

	// Create credentials securely using PowerShell
	execCmd := fmt.Sprintf(`
		$ErrorActionPreference = "Stop"
		$vmName = "%s"
		$username = "%s"
		$password = "%s"

		# Create credential object
		$securePassword = ConvertTo-SecureString $password -AsPlainText -Force
		$credential = New-Object System.Management.Automation.PSCredential ($username, $securePassword)

		# The script to execute in the VM
		$scriptContent = @'
%s
'@

		# Execute script in VM with retries
		$maxRetries = 10
		$retryCount = 0
		$retryDelay = 10

		while ($retryCount -lt $maxRetries) {
			try {
				Write-Output "Attempt $($retryCount + 1) of $maxRetries to connect to VM..."

				# Execute the script in the VM
				$result = Invoke-Command -VMName $vmName -Credential $credential -ScriptBlock {
					param($script)

					# Write script to temp file and execute it
					$tempScript = "$env:TEMP\configure-runner-$([guid]::NewGuid()).ps1"
					Set-Content -Path $tempScript -Value $script -Force

					try {
						& powershell.exe -ExecutionPolicy Bypass -NoProfile -File $tempScript 2>&1
						$exitCode = $LASTEXITCODE
						Remove-Item $tempScript -Force -ErrorAction SilentlyContinue

						if ($exitCode -ne 0) {
							throw "Script exited with code $exitCode"
						}
					} catch {
						Remove-Item $tempScript -Force -ErrorAction SilentlyContinue
						throw
					}
				} -ArgumentList $scriptContent

				# Output the result
				$result | ForEach-Object { Write-Output $_ }

				Write-Output "SCRIPT_EXECUTION_SUCCESS"
				break
			} catch {
				$retryCount++
				if ($retryCount -lt $maxRetries) {
					Write-Output "Connection failed: $_"
					Write-Output "Waiting $retryDelay seconds before retry..."
					Start-Sleep -Seconds $retryDelay
				} else {
					throw "Failed to execute script after $maxRetries attempts: $_"
				}
			}
		}
	`, vmName, h.config.HyperV.VMUsername, h.config.HyperV.VMPassword, escapedScript)

	output, err := h.RunPowerShell(execCmd)
	if err != nil {
		return fmt.Errorf("failed to execute script in VM: %w", err)
	}

	h.logger.Debug("Script execution output", "output", output)

	if !strings.Contains(output, "SCRIPT_EXECUTION_SUCCESS") {
		return fmt.Errorf("script execution did not complete successfully: %s", output)
	}

	h.logger.Info("Script executed successfully in VM", "vm_name", vmName)
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

		# Find and remove VMs matching the prefix followed by digits only
		# This ensures we only match numbered pool VMs like "github-runner-1", "github-runner-2"
		# and NOT other VMs like "github-runner-basic", "github-runner-template", etc.
		$vms = Get-VM | Where-Object { $_.Name -match "^$([regex]::Escape($namePrefix))\d+$" }
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

		# Find and remove orphaned VHDX files matching the prefix followed by digits only
		if (Test-Path $storagePath) {
			$vhdxFiles = Get-ChildItem -Path $storagePath -Filter "$namePrefix*.vhdx" -ErrorAction SilentlyContinue |
				Where-Object { $_.BaseName -match "^$([regex]::Escape($namePrefix))\d+$" }
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
