# GitHub Actions Runner Build Script
# Wrapper around Packer that uses hv-packer templates with runner customizations

param(
    [Parameter(Mandatory=$true)]
    [ValidateSet("basic", "enhanced")]
    [string]$BuildType,

    [switch]$ValidateOnly,

    [switch]$EnableLogging
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "GitHub Actions Runner Builder" -ForegroundColor Cyan
Write-Host "Build Type: $BuildType" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Set paths
$ScriptDir = $PSScriptRoot
$VariablesFile = Join-Path $ScriptDir "runner-customizations\variables\runner-$BuildType.pkvars.hcl"
$TemplateDir = Join-Path $ScriptDir "runner-customizations"

# Enable Packer logging if requested
if ($EnableLogging) {
    $env:PACKER_LOG = "1"
    $env:PACKER_LOG_PATH = Join-Path $ScriptDir "packer-$BuildType-$(Get-Date -Format 'yyyyMMdd-HHmmss').log"
    Write-Host "Logging enabled: $($env:PACKER_LOG_PATH)" -ForegroundColor Yellow
} else {
    $env:PACKER_LOG = "0"
}

# Verify files exist
if (-not (Test-Path $VariablesFile)) {
    Write-Host "ERROR: Variables file not found: $VariablesFile" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $TemplateDir)) {
    Write-Host "ERROR: Template directory not found: $TemplateDir" -ForegroundColor Red
    exit 1
}

# Verify hv-packer submodule is initialized
$HvPackerPath = Join-Path $ScriptDir "hv-packer"
if (-not (Test-Path (Join-Path $HvPackerPath "templates\hv_windows.pkr.hcl"))) {
    Write-Host "ERROR: hv-packer submodule not initialized" -ForegroundColor Red
    Write-Host "Run: git submodule update --init --recursive" -ForegroundColor Yellow
    exit 1
}

Write-Host "Configuration:" -ForegroundColor Cyan
Write-Host "  Variables: $VariablesFile" -ForegroundColor Gray
Write-Host "  Template Directory: $TemplateDir" -ForegroundColor Gray
Write-Host "  Build Type: $BuildType" -ForegroundColor Gray
Write-Host ""

# Change to packer directory so relative paths work
Push-Location $ScriptDir

try {
    # Determine the build name based on build type
    $BuildName = "runner-$BuildType.hyperv-iso.runner-$BuildType"

    # Validate
    Write-Host "Validating Packer configuration..." -ForegroundColor Yellow
    $validateCmd = "packer validate -only=`"$BuildName`" -var-file=`"$VariablesFile`" `"$TemplateDir`""
    Write-Host "  Command: $validateCmd" -ForegroundColor Gray

    Invoke-Expression $validateCmd
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: Packer validation failed" -ForegroundColor Red
        exit $LASTEXITCODE
    }

    Write-Host "Validation successful!" -ForegroundColor Green
    Write-Host ""

    # Build (unless validate-only)
    if ($ValidateOnly) {
        Write-Host "Validation-only mode: Skipping build" -ForegroundColor Yellow
    } else {
        Write-Host "Starting Packer build..." -ForegroundColor Yellow
        Write-Host "This will take 1-4 hours depending on build type and hardware" -ForegroundColor Yellow
        Write-Host ""

        $buildCmd = "packer build --force -only=`"$BuildName`" -var-file=`"$VariablesFile`" `"$TemplateDir`""
        Write-Host "  Command: $buildCmd" -ForegroundColor Gray
        Write-Host ""

        Invoke-Expression $buildCmd
        if ($LASTEXITCODE -ne 0) {
            Write-Host "ERROR: Packer build failed" -ForegroundColor Red
            exit $LASTEXITCODE
        }

        Write-Host ""
        Write-Host "========================================" -ForegroundColor Green
        Write-Host "Build completed successfully!" -ForegroundColor Green
        Write-Host "========================================" -ForegroundColor Green
    }
} finally {
    Pop-Location
}
