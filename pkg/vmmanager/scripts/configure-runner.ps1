# Configure GitHub Actions Runner
# This script is injected and executed by the orchestrator after VM creation
# It downloads, installs, configures the runner and creates a scheduled task for automatic startup

$ErrorActionPreference = "Stop"

Write-Host "=========================================="
Write-Host "GitHub Actions Runner Setup"
Write-Host "=========================================="

$runnerPath = "C:\actions-runner"
$configPath = "C:\runner-config.json"

# Verify config file exists
if (-not (Test-Path $configPath)) {
    throw "Runner configuration file not found at $configPath. The orchestrator should inject this before running this script."
}

# Step 1: Download and install GitHub Actions Runner if not already present
if (-not (Test-Path "$runnerPath\config.cmd")) {
    Write-Host ""
    Write-Host "Step 1: Installing GitHub Actions Runner..."
    Write-Host "--------------------------------------------"

    # Create runner directory
    Write-Host "Creating runner directory at $runnerPath..."
    New-Item -Path $runnerPath -ItemType Directory -Force | Out-Null
    Set-Location $runnerPath

    # Download the latest runner
    Write-Host "Downloading GitHub Actions Runner..."
    try {
        $latestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/actions/runner/releases/latest"
        $downloadUrl = $latestRelease.assets | Where-Object { $_.name -like "*win-x64*.zip" } | Select-Object -First 1 -ExpandProperty browser_download_url

        if (-not $downloadUrl) {
            throw "Could not find Windows x64 runner in latest release"
        }

        Write-Host "Downloading from: $downloadUrl"
        Invoke-WebRequest -Uri $downloadUrl -OutFile "actions-runner.zip" -UseBasicParsing
    } catch {
        throw "Failed to download GitHub Actions Runner: $_"
    }

    # Extract runner
    Write-Host "Extracting runner..."
    try {
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [System.IO.Compression.ZipFile]::ExtractToDirectory("$runnerPath\actions-runner.zip", $runnerPath)
    } catch {
        throw "Failed to extract runner: $_"
    }

    # Cleanup zip file
    Remove-Item -Path "$runnerPath\actions-runner.zip" -Force -ErrorAction SilentlyContinue

    # Verify extraction
    if (-not (Test-Path "$runnerPath\config.cmd")) {
        throw "Runner extraction failed - config.cmd not found"
    }

    Write-Host "GitHub Actions Runner installed successfully!"
} else {
    Write-Host ""
    Write-Host "Step 1: Runner Already Installed"
    Write-Host "--------------------------------------------"
    Write-Host "GitHub Actions Runner already present at $runnerPath"
}

Write-Host ""
Write-Host "Step 2: Reading Runner Configuration..."
Write-Host "--------------------------------------------"
try {
    $configJson = Get-Content -Path $configPath -Raw
    $config = $configJson | ConvertFrom-Json
} catch {
    throw "Failed to read configuration: $_"
}

Write-Host "Configuration loaded:"
Write-Host "  Organization: $($config.organization)"
Write-Host "  Repository: $($config.repository)"
Write-Host "  Name: $($config.name)"
Write-Host "  Labels: $($config.labels)"
if ($config.runner_group) {
    Write-Host "  Runner Group: $($config.runner_group)"
}

Write-Host ""
Write-Host "Step 3: Creating Startup Script..."
Write-Host "--------------------------------------------"
# Create startup script that will run on each boot
$startupScriptPath = "C:\actions-runner\startup.ps1"
$startupScript = @"
# GitHub Actions Runner Startup Script
# This script runs on VM boot and registers the runner with GitHub

`$ErrorActionPreference = "Stop"
`$runnerPath = "C:\actions-runner"
Set-Location `$runnerPath

Write-Host "Starting GitHub Actions Runner configuration..."

# Read runner configuration
`$configPath = "C:\runner-config.json"
if (-not (Test-Path `$configPath)) {
    Write-Error "Configuration file not found at `$configPath"
    exit 1
}

try {
    `$configJson = Get-Content -Path `$configPath -Raw
    `$config = `$configJson | ConvertFrom-Json
} catch {
    Write-Error "Failed to read configuration: `$_"
    exit 1
}

Write-Host "Configuration loaded for runner: `$(`$config.name)"

# Remove any existing runner configuration
if (Test-Path ".runner") {
    Write-Host "Removing existing runner configuration..."
    .\config.cmd remove --token `$(`$config.token)
}

# Configure runner
Write-Host "Configuring GitHub Actions runner..."
`$configArgs = @(
    "--unattended",
    "--url"
)

if (`$config.repository) {
    # Repository-level runner
    `$configArgs += "https://github.com/`$(`$config.organization)/`$(`$config.repository)"
} else {
    # Organization-level runner
    `$configArgs += "https://github.com/`$(`$config.organization)"
}

`$configArgs += @(
    "--token", `$config.token,
    "--name", `$config.name,
    "--labels", `$config.labels,
    "--ephemeral",
    "--disableupdate"
)

# Add runner group if specified (org-level runners only)
if (`$config.runner_group -and -not `$config.repository) {
    `$configArgs += @("--runnergroup", `$config.runner_group)
    Write-Host "Using runner group: `$(`$config.runner_group)"
}

& .\config.cmd @configArgs

if (`$LASTEXITCODE -ne 0) {
    Write-Error "Failed to configure runner (exit code: `$LASTEXITCODE)"
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
"@

# Write startup script
Set-Content -Path $startupScriptPath -Value $startupScript -Force -Encoding UTF8
Write-Host "Startup script created at $startupScriptPath"

Write-Host ""
Write-Host "Step 4: Creating Scheduled Task..."
Write-Host "--------------------------------------------"

# Remove existing task if present
$existingTask = Get-ScheduledTask -TaskName "GitHubActionsRunner" -ErrorAction SilentlyContinue
if ($existingTask) {
    Write-Host "Removing existing scheduled task..."
    Unregister-ScheduledTask -TaskName "GitHubActionsRunner" -Confirm:$false
}

# Create task components
$action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-ExecutionPolicy Bypass -NoProfile -File `"$startupScriptPath`""
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable

# Register the task
$task = Register-ScheduledTask -TaskName "GitHubActionsRunner" -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force

# Verify task was created
$task = Get-ScheduledTask -TaskName "GitHubActionsRunner" -ErrorAction Stop
Write-Host "Scheduled task created successfully:"
Write-Host "  Name: $($task.TaskName)"
Write-Host "  State: $($task.State)"
Write-Host "  Trigger: At Startup"
Write-Host "  User: SYSTEM"

Write-Host ""
Write-Host "Step 5: Starting Runner..."
Write-Host "--------------------------------------------"
Write-Host "Starting runner task for first execution..."
Start-ScheduledTask -TaskName "GitHubActionsRunner"

Write-Host ""
Write-Host "=========================================="
Write-Host "Setup Complete!"
Write-Host "=========================================="
Write-Host "The runner is now starting and will register with GitHub."
Write-Host "After the first job completes, the VM will shut down automatically."
Write-Host "The orchestrator will detect this and recreate the VM for the next job."
