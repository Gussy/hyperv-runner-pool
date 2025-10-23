# Configure GitHub Actions Runner
# This script is injected and executed by the orchestrator after VM creation
# It downloads, installs, configures, and runs the ephemeral runner directly

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

# Change to runner directory for configuration
Set-Location $runnerPath

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
if ($config.cache_url) {
    Write-Host "  Cache URL: $($config.cache_url)"
}

Write-Host ""
Write-Host "Step 3: Configuring Runner..."
Write-Host "--------------------------------------------"

# Remove any existing runner configuration
if (Test-Path ".runner") {
    Write-Host "Removing existing runner configuration..."
    .\config.cmd remove --token $config.token
}

# Configure runner
Write-Host "Registering runner with GitHub..."
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

# Add runner group if specified (org-level runners only)
if ($config.runner_group -and -not $config.repository) {
    $configArgs += @("--runnergroup", $config.runner_group)
    Write-Host "Using runner group: $($config.runner_group)"
}

& .\config.cmd @configArgs

if ($LASTEXITCODE -ne 0) {
    throw "Failed to configure runner (exit code: $LASTEXITCODE)"
}

Write-Host "Runner configured successfully!"

Write-Host ""
Write-Host "Step 4: Starting Runner..."
Write-Host "--------------------------------------------"
Write-Host "Running in ephemeral single-job mode..."
Write-Host "Runner will wait for a job, execute it, then exit."

# Patch runner for custom cache server if URL is provided
if ($config.cache_url) {
    Write-Host ""
    Write-Host "Configuring custom cache server..."
    Write-Host "  Cache URL: $($config.cache_url)"

    # Patch the runner binary to use custom cache server
    # GitHub's runner doesn't natively support custom ACTIONS_RESULTS_URL,
    # so we need to patch the Runner.Worker.dll binary
    #
    # This replaces the string "ACTIONS_RESULTS_URL" with "ACTIONS_RESULTS_ORL"
    # in the binary, which allows us to use CUSTOM_ACTIONS_RESULTS_URL env var
    # See: https://gha-cache-server.falcondev.io/getting-started

    $workerDllPath = "$runnerPath\bin\Runner.Worker.dll"

    if (Test-Path $workerDllPath) {
        Write-Host "  Patching runner binary for custom cache server..."

        try {
            # Read the binary file
            $bytes = [System.IO.File]::ReadAllBytes($workerDllPath)

            # Convert to string for pattern matching (using ASCII encoding)
            $content = [System.Text.Encoding]::ASCII.GetString($bytes)

            # Replace ACTIONS_RESULTS_URL with ACTIONS_RESULTS_ORL
            # This effectively disables the hardcoded URL check
            $oldPattern = "ACTIONS_RESULTS_URL"
            $newPattern = "ACTIONS_RESULTS_ORL"

            if ($content.Contains($oldPattern)) {
                $content = $content.Replace($oldPattern, $newPattern)

                # Convert back to bytes
                $patchedBytes = [System.Text.Encoding]::ASCII.GetBytes($content)

                # Write patched binary
                [System.IO.File]::WriteAllBytes($workerDllPath, $patchedBytes)

                Write-Host "  Runner binary patched successfully"

                # Now set the custom cache URL environment variable
                $env:CUSTOM_ACTIONS_RESULTS_URL = $config.cache_url
                Write-Host "  Custom cache server URL set: $($config.cache_url)"
            } else {
                Write-Host "  Runner appears to be already patched or incompatible"
                Write-Host "  Attempting to use cache server anyway..."
                $env:CUSTOM_ACTIONS_RESULTS_URL = $config.cache_url
            }
        } catch {
            Write-Host "  WARNING: Failed to patch runner binary: $_"
            Write-Host "  Cache server may not work correctly"
        }
    } else {
        Write-Host "  WARNING: Runner.Worker.dll not found at $workerDllPath"
        Write-Host "  Skipping runner patching"
    }
}

Write-Host ""

# Run the runner (this will block until job completes)
# Using --once flag to run a single job then exit
& .\run.cmd --once

Write-Host ""
Write-Host "=========================================="
Write-Host "Job Complete - Shutting Down"
Write-Host "=========================================="
Write-Host "Runner has completed its job and will shut down."
Write-Host "The orchestrator will detect this and recreate the VM."
Write-Host ""

# Give a brief moment for any final cleanup
Start-Sleep -Seconds 2

# Shutdown the VM - orchestrator will recreate it
Stop-Computer -Force
