# Hyper-V Runner Pool - Windows Startup Script

$ErrorActionPreference = "Stop"

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "  Hyper-V Runner Pool" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""

# Default to config.yaml in current directory
$ConfigFile = "config.yaml"

# Allow overriding config file via command line argument
if ($args.Count -gt 0) {
    $ConfigFile = $args[0]
}

# Check if config file exists
if (-not (Test-Path $ConfigFile)) {
    Write-Host ""
    Write-Host "ERROR: Configuration file not found: $ConfigFile" -ForegroundColor Red
    Write-Host ""
    Write-Host "Please create a config.yaml file based on config.example.yaml:" -ForegroundColor Yellow
    Write-Host "  1. Copy config.example.yaml to config.yaml"
    Write-Host "  2. Fill in your actual values:"
    Write-Host "     - github_pat: Your GitHub Personal Access Token"
    Write-Host "     - github_org: Your organization name"
    Write-Host "     - github_repo: Your repository name (or leave empty for org runners)"
    Write-Host ""
    Write-Host "Example:"
    Write-Host "  Copy-Item config.example.yaml config.yaml"
    Write-Host "  notepad config.yaml"
    Write-Host ""
    exit 1
}

Write-Host "Using configuration file: $ConfigFile" -ForegroundColor Yellow
Write-Host ""

# Check if binary exists
if (-not (Test-Path "hyperv-runner-pool.exe")) {
    Write-Host ""
    Write-Host "ERROR: hyperv-runner-pool.exe not found!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Please build the binary first:" -ForegroundColor Yellow
    Write-Host "  - On Windows: go build -o hyperv-runner-pool.exe"
    Write-Host "  - Cross-compile: GOOS=windows GOARCH=amd64 go build -o hyperv-runner-pool.exe"
    Write-Host ""
    Write-Host "Then copy it to this directory."
    Write-Host ""
    exit 1
}

# Check Hyper-V
Write-Host "Checking Hyper-V..." -ForegroundColor Yellow
try {
    $hypervFeature = Get-WindowsOptionalFeature -FeatureName Microsoft-Hyper-V-All -Online
    if ($hypervFeature.State -ne "Enabled") {
        Write-Host "  WARNING: Hyper-V is not enabled" -ForegroundColor Yellow
        Write-Host "  Please enable it with:"
        Write-Host "    Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All"
        Write-Host "    (Reboot required after enabling)"
    }
    else {
        Write-Host "  Hyper-V is enabled" -ForegroundColor Green
    }
}
catch {
    Write-Host "  WARNING: Could not check Hyper-V status: $_" -ForegroundColor Yellow
}

Write-Host ""

Write-Host "Starting runner pool with system tray icon..." -ForegroundColor Green
Write-Host "  - Look for the icon in your Windows system tray" -ForegroundColor Cyan
Write-Host "  - Right-click the icon to restart VMs or exit" -ForegroundColor Cyan
Write-Host "  - To run in console mode, use: --no-tray flag" -ForegroundColor Cyan
Write-Host ""

# Start the runner pool with config file
& .\hyperv-runner-pool.exe --config $ConfigFile
