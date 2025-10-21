# Hyper-V Runner Pool - Windows Startup Script

$ErrorActionPreference = "Stop"

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "  Hyper-V Runner Pool" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""

# Check if .env file exists
if (-not (Test-Path ".env")) {
    Write-Host ""
    Write-Host "ERROR: .env file not found!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Please create a .env file based on .env.example:" -ForegroundColor Yellow
    Write-Host "  1. Copy .env.example to .env"
    Write-Host "  2. Fill in your actual values:"
    Write-Host "     - GITHUB_PAT: Your GitHub Personal Access Token"
    Write-Host "     - GITHUB_ORG: Your organization name"
    Write-Host "     - GITHUB_REPO: Your repository name (or leave empty for org runners)"
    Write-Host ""
    Write-Host "Example:"
    Write-Host "  Copy-Item .env.example .env"
    Write-Host "  notepad .env"
    Write-Host ""
    exit 1
}

# Load environment variables from .env file
Write-Host "Loading configuration from .env..." -ForegroundColor Yellow
Get-Content ".env" | ForEach-Object {
    if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
        $name = $matches[1].Trim()
        $value = $matches[2].Trim()
        [Environment]::SetEnvironmentVariable($name, $value, "Process")
        Write-Host "  $name = $value" -ForegroundColor Gray
    }
}

Write-Host ""

# Validate required environment variables
$required = @("GITHUB_PAT", "GITHUB_ORG")
$missing = @()

foreach ($var in $required) {
    if (-not [Environment]::GetEnvironmentVariable($var)) {
        $missing += $var
    }
}

if ($missing.Count -gt 0) {
    Write-Host ""
    Write-Host "ERROR: Missing required environment variables in .env:" -ForegroundColor Red
    Write-Host "  $($missing -join ', ')" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Please update your .env file with these values." -ForegroundColor Yellow
    Write-Host ""
    exit 1
}

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
    } else {
        Write-Host "  Hyper-V is enabled" -ForegroundColor Green
    }
} catch {
    Write-Host "  WARNING: Could not check Hyper-V status: $_" -ForegroundColor Yellow
}

Write-Host ""

# Check required directories (use environment variables or defaults)
$templatePath = [Environment]::GetEnvironmentVariable("VM_TEMPLATE_PATH")
$storagePath = [Environment]::GetEnvironmentVariable("VM_STORAGE_PATH")

# Use defaults if not specified in environment
if (-not $templatePath) {
    $templatePath = "vms\templates\runner-template.vhdx"
}

if (-not $storagePath) {
    $storagePath = "vms\storage"
}

Write-Host "Checking VM configuration..." -ForegroundColor Yellow
Write-Host "  Template path: $templatePath" -ForegroundColor Gray
Write-Host "  Storage path:  $storagePath" -ForegroundColor Gray

# Extract directory from template path
$templateDir = Split-Path $templatePath -Parent

# Ensure directories exist
if ($templateDir -and -not (Test-Path $templateDir)) {
    Write-Host "  Creating template directory: $templateDir" -ForegroundColor Yellow
    New-Item -Path $templateDir -ItemType Directory -Force | Out-Null
}

if (-not (Test-Path $storagePath)) {
    Write-Host "  Creating storage directory: $storagePath" -ForegroundColor Yellow
    New-Item -Path $storagePath -ItemType Directory -Force | Out-Null
}

# Check for template file
if (-not (Test-Path $templatePath)) {
    Write-Host ""
    Write-Host "WARNING: Template file not found: $templatePath" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "You need to build the VM template with Packer first:" -ForegroundColor Yellow
    Write-Host "  cd packer"
    Write-Host "  packer init ."
    Write-Host "  packer build windows-runner.pkr.hcl"
    Write-Host ""
    Write-Host "Then copy the resulting VHDX to: $templatePath"
    Write-Host ""
    Write-Host "Example:"
    Write-Host "  Copy the .vhdx file from output-windows-runner\Virtual Hard Disks\"
    Write-Host "  to vms\templates\runner-template.vhdx"
    Write-Host ""
}

Write-Host ""
Write-Host "Starting runner pool..." -ForegroundColor Green
Write-Host ""

# Start the runner pool
& .\hyperv-runner-pool.exe
