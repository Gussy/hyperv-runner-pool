# Hyper-V Runner Pool - Windows Startup Script

$ErrorActionPreference = "Stop"

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "  Hyper-V Runner Pool" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""

# Check if .env file exists
if (-not (Test-Path ".env")) {
    Write-Error @"
.env file not found!

Please create a .env file based on .env.example:
  1. Copy .env.example to .env
  2. Fill in your actual values:
     - GITHUB_PAT: Your GitHub Personal Access Token
     - GITHUB_ORG: Your organization name
     - GITHUB_REPO: Your repository name (or leave empty for org runners)
     - WEBHOOK_SECRET: Your webhook secret
     - ORCHESTRATOR_IP: Your machine's IP address

Example:
  Copy-Item .env.example .env
  notepad .env
"@
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
$required = @("GITHUB_PAT", "GITHUB_ORG", "WEBHOOK_SECRET")
$missing = @()

foreach ($var in $required) {
    if (-not [Environment]::GetEnvironmentVariable($var)) {
        $missing += $var
    }
}

if ($missing.Count -gt 0) {
    Write-Error @"
Missing required environment variables in .env:
  $($missing -join ', ')

Please update your .env file with these values.
"@
    exit 1
}

# Check if binary exists
if (-not (Test-Path "hyperv-runner-pool.exe")) {
    Write-Error @"
hyperv-runner-pool.exe not found!

Please build the binary first:
  - On macOS: GOOS=windows GOARCH=amd64 go build -o hyperv-runner-pool.exe
  - On Windows: go build -o hyperv-runner-pool.exe

Then copy it to this directory.
"@
    exit 1
}

# Check Hyper-V
Write-Host "Checking Hyper-V..." -ForegroundColor Yellow
try {
    $hypervFeature = Get-WindowsOptionalFeature -FeatureName Microsoft-Hyper-V-All -Online
    if ($hypervFeature.State -ne "Enabled") {
        Write-Warning "Hyper-V is not enabled. Please enable it with:"
        Write-Warning "  Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All"
        Write-Warning "  (Reboot required after enabling)"
    } else {
        Write-Host "  Hyper-V is enabled" -ForegroundColor Green
    }
} catch {
    Write-Warning "Could not check Hyper-V status: $_"
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
    Write-Warning "Template directory not found: $templateDir"
    Write-Host "  Creating directory..."
    New-Item -Path $templateDir -ItemType Directory -Force | Out-Null
}

if (-not (Test-Path $storagePath)) {
    Write-Warning "VM storage directory not found: $storagePath"
    Write-Host "  Creating directory..."
    New-Item -Path $storagePath -ItemType Directory -Force | Out-Null
}

# Check for template file
if (-not (Test-Path $templatePath)) {
    Write-Warning @"
Template file not found: $templatePath

You need to build the VM template with Packer first:
  cd packer
  packer init .
  packer build windows-runner.pkr.hcl

Then copy the resulting VHDX to:
  $templatePath

Example (from packer directory):
  Copy-Item ".\output-windows-runner\Virtual Hard Disks\*.vhdx" "..\vms\templates\runner-template.vhdx"
"@
}

Write-Host ""
Write-Host "Starting runner pool..." -ForegroundColor Green
Write-Host ""

# Start the runner pool
& .\hyperv-runner-pool.exe
