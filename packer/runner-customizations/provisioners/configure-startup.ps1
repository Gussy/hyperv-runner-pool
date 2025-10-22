# Configure VM Startup Script
Write-Host "Configuring VM startup script..."

# Create startup script that VMs will run on boot
$startupScriptPath = "C:\actions-runner\startup.ps1"
$startupScript = @'
# GitHub Actions Runner Startup Script
# This script runs on VM boot and registers the runner with GitHub

$ErrorActionPreference = "Stop"
$runnerPath = "C:\actions-runner"
Set-Location $runnerPath

# Wait for disk to be ready
Write-Host "Waiting for system initialization..."
Start-Sleep -Seconds 5

# Read runner configuration from injected file
$configPath = "C:\runner-config.json"
Write-Host "Reading runner configuration from $configPath..."

if (-not (Test-Path $configPath)) {
    Write-Error "Configuration file not found at $configPath"
    exit 1
}

try {
    $configJson = Get-Content -Path $configPath -Raw
    $config = $configJson | ConvertFrom-Json
} catch {
    Write-Error "Failed to read configuration: $_"
    exit 1
}

Write-Host "Configuration loaded:"
Write-Host "  Organization: $($config.organization)"
Write-Host "  Repository: $($config.repository)"
Write-Host "  Name: $($config.name)"
Write-Host "  Labels: $($config.labels)"

# Remove any existing runner configuration
if (Test-Path ".runner") {
    Write-Host "Removing existing runner configuration..."
    .\config.cmd remove --token $($config.token)
}

# Configure runner
Write-Host "Configuring GitHub Actions runner..."
$configArgs = @(
    "--unattended",
    "--url"
)

if ($config.repository) {
    # Repository-level runner
    $configArgs += "https://github.com/$($config.organization)/$($config.repository)"
} else {
    # Organization-level runner
    $configArgs += "https://github.com/$($config.organization)"
}

$configArgs += @(
    "--token", $config.token,
    "--name", $config.name,
    "--labels", $config.labels,
    "--ephemeral",
    "--disableupdate"
)

& .\config.cmd @configArgs

if ($LASTEXITCODE -ne 0) {
    Write-Error "Failed to configure runner (exit code: $LASTEXITCODE)"
    exit 1
}

Write-Host "Runner configured successfully!"

# Run the runner (this will block until job completes)
# Using --once flag to run a single job then exit
Write-Host "Starting runner (single job mode)..."
& .\run.cmd --once

# When runner completes, shutdown the VM
# The orchestrator will detect the shutdown and recreate the VM
Write-Host "Job completed. Shutting down VM..."
Start-Sleep -Seconds 5
Stop-Computer -Force
'@

# Write startup script
Set-Content -Path $startupScriptPath -Value $startupScript -Force
Write-Host "Startup script created at: $startupScriptPath"

# Create scheduled task to run startup script on boot
Write-Host "Creating scheduled task for startup..."
try {
    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-ExecutionPolicy Bypass -File `"$startupScriptPath`""
    $trigger = New-ScheduledTaskTrigger -AtStartup
    $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable

    # Unregister existing task if it exists
    $existingTask = Get-ScheduledTask -TaskName "GitHubActionsRunner" -ErrorAction SilentlyContinue
    if ($existingTask) {
        Write-Host "Removing existing scheduled task..."
        Unregister-ScheduledTask -TaskName "GitHubActionsRunner" -Confirm:$false
    }

    # Register the new task
    $task = Register-ScheduledTask -TaskName "GitHubActionsRunner" -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force
    Write-Host "Scheduled task 'GitHubActionsRunner' created successfully"

    # Verify scheduled task is enabled (it should be by default)
    $task = Get-ScheduledTask -TaskName "GitHubActionsRunner" -ErrorAction SilentlyContinue
    if ($task) {
        Write-Host "Scheduled task verified: $($task.TaskName) - State: $($task.State)"
        if ($task.State -ne "Ready") {
            Write-Warning "Task state is $($task.State) instead of Ready. This may cause issues."
        }
    } else {
        throw "Scheduled task verification failed"
    }
} catch {
    Write-Error "Failed to create scheduled task: $_"
    exit 1
}

Write-Host "Startup configuration complete!"
