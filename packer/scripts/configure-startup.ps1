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

# Wait for network to be ready
Write-Host "Waiting for network..."
Start-Sleep -Seconds 10

# Get VM name from computer name
$vmName = $env:COMPUTERNAME

# Get orchestrator IP from environment or use default
$orchestratorIP = if ($env:ORCHESTRATOR_IP) { $env:ORCHESTRATOR_IP } else { "localhost" }
$configUrl = "http://${orchestratorIP}:8080/api/runner-config/${vmName}"
$completeUrl = "http://${orchestratorIP}:8080/api/runner-complete/${vmName}"

Write-Host "VM Name: $vmName"
Write-Host "Orchestrator: $orchestratorIP"

# Fetch configuration from orchestrator
Write-Host "Fetching runner configuration from orchestrator..."
try {
    $config = Invoke-RestMethod -Uri $configUrl -Method Get -TimeoutSec 30
} catch {
    Write-Error "Failed to fetch configuration: $_"
    exit 1
}

Write-Host "Configuration received:"
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
Write-Host "Starting runner..."
& .\run.cmd

# When runner completes (job done), notify orchestrator
Write-Host "Job completed. Notifying orchestrator..."
try {
    Invoke-RestMethod -Uri $completeUrl -Method Post -TimeoutSec 30
    Write-Host "Orchestrator notified. VM will be recreated."
} catch {
    Write-Warning "Failed to notify orchestrator: $_"
}

# Shutdown the VM (orchestrator will destroy it)
Write-Host "Shutting down VM..."
Start-Sleep -Seconds 5
Stop-Computer -Force
'@

# Write startup script
Set-Content -Path $startupScriptPath -Value $startupScript -Force
Write-Host "Startup script created at: $startupScriptPath"

# Create scheduled task to run startup script on boot
Write-Host "Creating scheduled task for startup..."
$action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-ExecutionPolicy Bypass -File `"$startupScriptPath`""
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable

Register-ScheduledTask -TaskName "GitHubActionsRunner" -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force

Write-Host "Scheduled task 'GitHubActionsRunner' created successfully"

# Verify scheduled task
$task = Get-ScheduledTask -TaskName "GitHubActionsRunner" -ErrorAction SilentlyContinue
if ($task) {
    Write-Host "Scheduled task verified: $($task.TaskName) - State: $($task.State)"
} else {
    Write-Error "Failed to create scheduled task"
    exit 1
}

Write-Host "Startup configuration complete!"
