# Hyper-V Runner Pool Service Installer
# Run with Administrator privileges
# Uses NSSM (Non-Sucking Service Manager) to run the Go app as a Windows service

$ErrorActionPreference = "Stop"

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host 'ERROR: This script requires Administrator privileges' -ForegroundColor Red
    Write-Host 'Please run PowerShell as Administrator and try again' -ForegroundColor Yellow
    exit 1
}

$installPath = 'C:\ProgramData\hyperv-runner-pool'
$serviceName = 'hyperv-runner-pool'
$exePath = Join-Path $installPath 'hyperv-runner-pool.exe'
$configPath = Join-Path $installPath 'config.yaml'

Write-Host '==================================================' -ForegroundColor Cyan
Write-Host '  Installing Hyper-V Runner Pool Service' -ForegroundColor Cyan
Write-Host '==================================================' -ForegroundColor Cyan
Write-Host ''

# Check if NSSM is installed
$nssmPath = Get-Command nssm.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
if (-not $nssmPath) {
    Write-Host 'ERROR: NSSM (Non-Sucking Service Manager) is not installed' -ForegroundColor Red
    Write-Host ''
    Write-Host 'NSSM is required to run the application as a Windows service.' -ForegroundColor Yellow
    Write-Host 'Please install it using Chocolatey:' -ForegroundColor Yellow
    Write-Host ''
    Write-Host '  choco install nssm -y' -ForegroundColor Cyan
    Write-Host ''
    Write-Host 'Or download manually from: https://nssm.cc' -ForegroundColor Gray
    Write-Host ''
    exit 1
}

Write-Host "Found NSSM at: $nssmPath" -ForegroundColor Green
Write-Host ''

# Check if executable exists in current directory
if (-not (Test-Path 'hyperv-runner-pool.exe')) {
    Write-Host 'ERROR: hyperv-runner-pool.exe not found in current directory' -ForegroundColor Red
    Write-Host 'Please build it first: task build' -ForegroundColor Yellow
    exit 1
}

# Check if service exists and stop/remove it
$existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($existingService) {
    Write-Host 'Found existing service, removing it...' -ForegroundColor Yellow

    # Temporarily allow errors to continue
    $oldErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'

    # Try to stop the service
    Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2

    # Try NSSM remove first
    & $nssmPath remove $serviceName confirm 2>&1 | Out-Null
    Start-Sleep -Seconds 1

    # Also try sc.exe delete as a fallback (sometimes works when NSSM doesn't)
    sc.exe delete $serviceName 2>&1 | Out-Null

    # Restore error action preference
    $ErrorActionPreference = $oldErrorActionPreference

    # Wait for Windows to fully delete the service (can take several seconds)
    Write-Host 'Waiting for service deletion to complete' -NoNewline -ForegroundColor Yellow
    $maxWait = 15
    $waited = 0
    while ((Get-Service -Name $serviceName -ErrorAction SilentlyContinue) -and ($waited -lt $maxWait)) {
        Start-Sleep -Seconds 1
        $waited++
        Write-Host "." -NoNewline -ForegroundColor Yellow
    }
    Write-Host ""

    # Check if still exists
    if (Get-Service -Name $serviceName -ErrorAction SilentlyContinue) {
        Write-Host ''
        Write-Host 'ERROR: Service is marked for deletion but not fully removed yet.' -ForegroundColor Red
        Write-Host 'This happens when the service process is still running.' -ForegroundColor Yellow
        Write-Host ''
        Write-Host 'Please try one of these options:' -ForegroundColor Yellow
        Write-Host '  1. Wait 30 seconds and run the command again' -ForegroundColor Cyan
        Write-Host '  2. Restart your computer to force cleanup' -ForegroundColor Cyan
        Write-Host '  3. Manually kill any hyperv-runner-pool.exe processes in Task Manager' -ForegroundColor Cyan
        Write-Host ''
        exit 1
    }

    Write-Host 'Service removed successfully' -ForegroundColor Green
    Start-Sleep -Seconds 1
}

# Create installation directory
Write-Host "Creating installation directory: $installPath" -ForegroundColor Yellow
if (-not (Test-Path $installPath)) {
    New-Item -ItemType Directory -Path $installPath -Force | Out-Null
}

# Copy executable
Write-Host 'Copying executable...' -ForegroundColor Yellow
Copy-Item -Path 'hyperv-runner-pool.exe' -Destination $exePath -Force

# Copy config.yaml if it exists
if (Test-Path 'config.yaml') {
    Write-Host 'Copying config.yaml...' -ForegroundColor Yellow
    Copy-Item -Path 'config.yaml' -Destination $configPath -Force
}
else {
    Write-Host 'WARNING: config.yaml not found in current directory' -ForegroundColor Yellow
    Write-Host "You will need to create it at: $configPath" -ForegroundColor Yellow
    Write-Host 'Use config.example.yaml as a template' -ForegroundColor Yellow
}

# Install service using NSSM (running as LocalSystem for Hyper-V permissions)
Write-Host 'Installing Windows service with NSSM...' -ForegroundColor Yellow

# Install the service
& $nssmPath install $serviceName $exePath --config $configPath

# Configure service
& $nssmPath set $serviceName DisplayName "Hyper-V Runner Pool"
& $nssmPath set $serviceName Description "Manages a pool of ephemeral Hyper-V VMs for GitHub Actions runners"
& $nssmPath set $serviceName Start SERVICE_AUTO_START
& $nssmPath set $serviceName ObjectName LocalSystem

# Set the working directory to the install path
& $nssmPath set $serviceName AppDirectory $installPath

# Note: We don't configure NSSM stdout/stderr logging because the application
# handles its own logging via the log_directory config option

# Start service
Write-Host 'Starting service...' -ForegroundColor Yellow
& $nssmPath start $serviceName

# Wait a moment and check status
Start-Sleep -Seconds 2
$service = Get-Service -Name $serviceName

Write-Host ''
Write-Host '==================================================' -ForegroundColor Green
Write-Host '  Installation Complete!' -ForegroundColor Green
Write-Host '==================================================' -ForegroundColor Green
Write-Host ''
Write-Host "Service Name:    $serviceName" -ForegroundColor Cyan
Write-Host "Service Status:  $($service.Status)" -ForegroundColor Cyan
Write-Host "Startup Type:    Automatic (starts on boot)" -ForegroundColor Cyan
Write-Host "Install Path:    $installPath" -ForegroundColor Cyan
Write-Host "Config Path:     $configPath" -ForegroundColor Cyan
Write-Host ''
Write-Host 'IMPORTANT: Configure logging in your config.yaml:' -ForegroundColor Yellow
Write-Host '  logging:' -ForegroundColor Gray
Write-Host "    directory: $installPath\logs" -ForegroundColor Gray
Write-Host ''
Write-Host 'Useful commands:' -ForegroundColor Yellow
Write-Host "  Stop service:    nssm stop $serviceName" -ForegroundColor Gray
Write-Host "  Start service:   nssm start $serviceName" -ForegroundColor Gray
Write-Host "  Restart service: nssm restart $serviceName" -ForegroundColor Gray
Write-Host "  Service status:  nssm status $serviceName" -ForegroundColor Gray
Write-Host "  Remove service:  nssm remove $serviceName confirm" -ForegroundColor Gray
Write-Host ''
