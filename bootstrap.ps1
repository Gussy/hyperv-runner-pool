# Windows Development Environment Bootstrap Script
# Sets up all required tools for Hyper-V Runner Pool development on Windows

#Requires -RunAsAdministrator

$ErrorActionPreference = "Stop"

# Color functions for better output
function Write-Step {
    param([string]$Message)
    Write-Host "`n$Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "  [OK] $Message" -ForegroundColor Green
}

function Write-Info {
    param([string]$Message)
    Write-Host "  [INFO] $Message" -ForegroundColor Yellow
}

function Write-Error-Custom {
    param([string]$Message)
    Write-Host "  [ERROR] $Message" -ForegroundColor Red
}

# Banner
Write-Host ""
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host "  Hyper-V Runner Pool - Windows Bootstrap" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host ""

# Check if running as Administrator
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Error-Custom "This script must be run as Administrator!"
    Write-Host ""
    Write-Host "  Please right-click PowerShell and select 'Run as Administrator'" -ForegroundColor Yellow
    exit 1
}

Write-Success "Running with Administrator privileges"

# Function to check if a command exists
function Test-Command {
    param([string]$Command)
    $null -ne (Get-Command $Command -ErrorAction SilentlyContinue)
}

# Function to get version of a command
function Get-CommandVersion {
    param(
        [string]$Command,
        [string]$VersionArg = "--version"
    )
    try {
        $output = & $Command $VersionArg 2>&1

        # For goreleaser, find the line with "version" or "GitVersion" in it
        if ($Command -eq "goreleaser") {
            $versionLine = $output | Where-Object { $_ -match 'version|GitVersion' } | Select-Object -First 1
            if ($versionLine) {
                # Extract version number (e.g., "version: 1.2.3" or "GitVersion: 1.2.3")
                if ($versionLine -match '[\d]+\.[\d]+\.[\d]+') {
                    return $matches[0]
                }
                return $versionLine.Trim()
            }
        }

        # For other commands, just return the first line
        return ($output | Select-Object -First 1)
    }
    catch {
        return "Unknown"
    }
}

# Step 1: Install Chocolatey
if (Test-Command "choco") {
    $chocoVersion = Get-CommandVersion "choco" "-v"
    Write-Success "Chocolatey is already installed (version $chocoVersion)"
}
else {
    Write-Info "Installing Chocolatey..."
    try {
        Set-ExecutionPolicy Bypass -Scope Process -Force
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
        Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

        Write-Success "Chocolatey installed successfully"
    }
    catch {
        Write-Error-Custom "Failed to install Chocolatey: $_"
        exit 1
    }
}

# Step 2: Install Go
if (Test-Command "go") {
    $goVersion = Get-CommandVersion "go" "version"
    Write-Success "Go is already installed ($goVersion)"
}
else {
    Write-Info "Installing Go 1.25.3..."
    try {
        choco install golang --version=1.25.3 -y

        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

        if (Test-Command "go") {
            $goVersion = Get-CommandVersion "go" "version"
            Write-Success "Go installed successfully ($goVersion)"
        }
        else {
            Write-Error-Custom "Go installation completed but 'go' command not found. You may need to restart your terminal."
        }
    }
    catch {
        Write-Error-Custom "Failed to install Go: $_"
        Write-Info "Continuing with other installations..."
    }
}

# Step 3: Install Git
if (Test-Command "git") {
    $gitVersion = Get-CommandVersion "git" "--version"
    Write-Success "Git is already installed ($gitVersion)"
}
else {
    Write-Info "Installing Git..."
    try {
        choco install git -y

        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

        if (Test-Command "git") {
            $gitVersion = Get-CommandVersion "git" "--version"
            Write-Success "Git installed successfully ($gitVersion)"
        }
        else {
            Write-Error-Custom "Git installation completed but 'git' command not found. You may need to restart your terminal."
        }
    }
    catch {
        Write-Error-Custom "Failed to install Git: $_"
        Write-Info "Continuing with other installations..."
    }
}

# Step 4: Install Task
if (Test-Command "task") {
    $taskVersion = Get-CommandVersion "task" "--version"
    Write-Success "Task is already installed ($taskVersion)"
}
else {
    Write-Info "Installing Task..."
    try {
        choco install go-task -y

        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

        if (Test-Command "task") {
            $taskVersion = Get-CommandVersion "task" "--version"
            Write-Success "Task installed successfully ($taskVersion)"
        }
        else {
            Write-Error-Custom "Task installation completed but 'task' command not found. You may need to restart your terminal."
        }
    }
    catch {
        Write-Error-Custom "Failed to install Task: $_"
        Write-Info "Continuing with other installations..."
    }
}

# Step 5: Install Packer
if (Test-Command "packer") {
    $packerVersion = Get-CommandVersion "packer" "version"
    Write-Success "Packer is already installed ($packerVersion)"
}
else {
    Write-Info "Installing Packer..."
    try {
        choco install packer -y

        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

        if (Test-Command "packer") {
            $packerVersion = Get-CommandVersion "packer" "version"
            Write-Success "Packer installed successfully ($packerVersion)"
        }
        else {
            Write-Error-Custom "Packer installation completed but 'packer' command not found. You may need to restart your terminal."
        }
    }
    catch {
        Write-Error-Custom "Failed to install Packer: $_"
        Write-Info "Continuing with other installations..."
    }
}

# Step 6: Install GoReleaser
if (Test-Command "goreleaser") {
    $goreleaserVersion = Get-CommandVersion "goreleaser" "--version"
    Write-Success "GoReleaser is already installed ($goreleaserVersion)"
}
else {
    Write-Info "Installing GoReleaser..."
    try {
        choco install goreleaser -y

        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

        if (Test-Command "goreleaser") {
            $goreleaserVersion = Get-CommandVersion "goreleaser" "--version"
            Write-Success "GoReleaser installed successfully ($goreleaserVersion)"
        }
        else {
            Write-Error-Custom "GoReleaser installation completed but 'goreleaser' command not found. You may need to restart your terminal."
        }
    }
    catch {
        Write-Error-Custom "Failed to install GoReleaser: $_"
        Write-Info "Continuing with other installations..."
    }
}

# Step 7: Check Hyper-V
try {
    $hypervFeature = Get-WindowsOptionalFeature -FeatureName Microsoft-Hyper-V-All -Online

    if ($hypervFeature.State -eq "Enabled") {
        Write-Success "Hyper-V is already enabled"
    }
    else {
        Write-Info "Enabling Hyper-V..."
        Write-Host "  NOTE: A system reboot will be required after enabling Hyper-V" -ForegroundColor Yellow

        $confirmation = Read-Host "  Do you want to enable Hyper-V now? (y/n)"
        if ($confirmation -eq 'y' -or $confirmation -eq 'Y') {
            Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All -NoRestart
            Write-Success "Hyper-V has been enabled"
            Write-Host "  IMPORTANT: You must reboot your computer for Hyper-V to work" -ForegroundColor Red
            $rebootNow = Read-Host "  Do you want to reboot now? (y/n)"
            if ($rebootNow -eq 'y' -or $rebootNow -eq 'Y') {
                Write-Host "  Rebooting in 10 seconds... (Press Ctrl+C to cancel)" -ForegroundColor Yellow
                Start-Sleep -Seconds 10
                Restart-Computer -Force
            }
        }
        else {
            Write-Info "Skipping Hyper-V enablement. You can enable it later with:"
            Write-Host "    Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All" -ForegroundColor Gray
        }
    }
}
catch {
    Write-Error-Custom "Could not check Hyper-V status: $_"
    Write-Info "You may need to enable Hyper-V manually for production use"
}

# Step 8: Create directory structure
$directories = @(
    "vms\templates",
    "vms\storage"
)

foreach ($dir in $directories) {
    if (Test-Path $dir) {
        Write-Success "Directory already exists: $dir"
    }
    else {
        try {
            New-Item -Path $dir -ItemType Directory -Force | Out-Null
            Write-Success "Created directory: $dir"
        }
        catch {
            Write-Error-Custom "Failed to create directory $dir : $_"
        }
    }
}

Write-Host ""
Write-Info "VM storage is now configured in the local repository:"
Write-Host "  Template: .\vms\templates\runner-template.vhdx" -ForegroundColor Gray
Write-Host "  Storage:  .\vms\storage\" -ForegroundColor Gray
Write-Host ""
Write-Info "To use a different location, set these environment variables in .env:"
Write-Host "  VM_TEMPLATE_PATH=C:\your\custom\path\runner-template.vhdx" -ForegroundColor Gray
Write-Host "  VM_STORAGE_PATH=C:\your\custom\storage\path" -ForegroundColor Gray

# Step 9: Setup environment file
if (Test-Path ".env") {
    Write-Success ".env file already exists"
}
else {
    if (Test-Path ".env.example") {
        Copy-Item ".env.example" ".env"
        Write-Success "Created .env from .env.example"
        Write-Info "IMPORTANT: Edit .env file and fill in your actual values!"
    }
    else {
        Write-Info ".env.example not found, skipping .env creation"
    }
}

# Step 10: Initialize Go modules
if (Test-Path "go.mod") {
    if (Test-Command "go") {
        try {
            Write-Info "Running go mod download..."
            go mod download
            Write-Info "Running go mod tidy..."
            go mod tidy
            Write-Success "Go dependencies downloaded successfully"
        }
        catch {
            Write-Error-Custom "Failed to download Go dependencies: $_"
            Write-Info "You can run 'go mod download' manually later"
        }
    }
    else {
        Write-Info "Go command not available, skipping dependency download"
    }
}
else {
    Write-Info "go.mod not found in current directory, skipping"
}

# Summary
Write-Host ""
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host "  Bootstrap Complete!" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host ""

Write-Host "Installed tools:" -ForegroundColor Green
$tools = @(
    @{Name = "Chocolatey"; Command = "choco"; Arg = "-v" },
    @{Name = "Go"; Command = "go"; Arg = "version" },
    @{Name = "Git"; Command = "git"; Arg = "--version" },
    @{Name = "Task"; Command = "task"; Arg = "--version" },
    @{Name = "Packer"; Command = "packer"; Arg = "version" },
    @{Name = "GoReleaser"; Command = "goreleaser"; Arg = "--version" }
)

foreach ($tool in $tools) {
    if (Test-Command $tool.Command) {
        $version = Get-CommandVersion $tool.Command $tool.Arg
        Write-Host "  [OK] $($tool.Name): $version" -ForegroundColor Gray
    }
    else {
        Write-Host "  [X] $($tool.Name): Not available" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host ""
Write-Host "  1. If Hyper-V was just enabled, REBOOT your computer" -ForegroundColor White
Write-Host ""
Write-Host "  2. Edit .env file with your actual values:" -ForegroundColor White
Write-Host "       notepad .env" -ForegroundColor Gray
Write-Host ""
Write-Host "  3. View available development tasks:" -ForegroundColor White
Write-Host "       task --list" -ForegroundColor Gray
Write-Host ""
Write-Host "  4. Run tests:" -ForegroundColor White
Write-Host "       task test" -ForegroundColor Gray
Write-Host ""
Write-Host "  5. Build the Windows binary:" -ForegroundColor White
Write-Host "       task build" -ForegroundColor Gray
Write-Host ""
Write-Host "  6. For production: Build VM template with Packer:" -ForegroundColor White
Write-Host "       cd packer" -ForegroundColor Gray
Write-Host "       packer init ." -ForegroundColor Gray
Write-Host "       packer build windows-runner.pkr.hcl" -ForegroundColor Gray
Write-Host "       Copy vhdx file to templates directory" -ForegroundColor Gray
Write-Host ""
Write-Host "  7. For more information, see README.md" -ForegroundColor White
Write-Host ""
