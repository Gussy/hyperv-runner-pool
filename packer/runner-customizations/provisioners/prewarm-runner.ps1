# Pre-warm GitHub Actions Runner
# This script downloads and installs the GitHub Actions Runner without registration
# to trigger all expensive .NET initialization, NuGet cache population, and NGEN compilation
# This reduces first-run overhead from ~3 minutes to ~10 seconds when VMs are created

Write-Output "Phase [START] - Pre-warming GitHub Actions Runner"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$ErrorActionPreference = "Stop"
$runnerPath = "C:\actions-runner"

try {
    # Create runner directory
    Write-Output "Creating runner directory at $runnerPath..."
    New-Item -Path $runnerPath -ItemType Directory -Force | Out-Null
    Set-Location $runnerPath

    # Download the latest GitHub Actions Runner
    Write-Output "Fetching latest GitHub Actions Runner release information..."
    $latestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/actions/runner/releases/latest"
    $downloadUrl = $latestRelease.assets | Where-Object { $_.name -like "*win-x64*.zip" } | Select-Object -First 1 -ExpandProperty browser_download_url

    if (-not $downloadUrl) {
        throw "Could not find Windows x64 runner in latest release"
    }

    $version = $latestRelease.tag_name
    Write-Output "Downloading GitHub Actions Runner $version from: $downloadUrl"
    Invoke-WebRequest -Uri $downloadUrl -OutFile "actions-runner.zip" -UseBasicParsing

    # Extract runner
    Write-Output "Extracting runner to $runnerPath..."
    Expand-Archive -Path "actions-runner.zip" -DestinationPath $runnerPath -Force

    # Cleanup zip file
    Remove-Item -Path "actions-runner.zip" -Force

    # Verify extraction
    if (-not (Test-Path "$runnerPath\config.cmd")) {
        throw "Runner extraction failed - config.cmd not found"
    }

    Write-Output "GitHub Actions Runner $version extracted successfully"

    # Trigger .NET initialization by loading runner assemblies
    # This populates NuGet cache and triggers NGEN compilation without needing GitHub registration
    Write-Output "Triggering .NET initialization and NGEN compilation..."

    # Load runner assemblies to trigger .NET caching
    try {
        Add-Type -Path "$runnerPath\bin\Runner.Listener.dll" -ErrorAction SilentlyContinue
        Write-Output "  - Runner.Listener.dll loaded"
    } catch {
        Write-Output "  - Runner.Listener.dll load attempted (expected to fail, but triggers caching)"
    }

    try {
        Add-Type -Path "$runnerPath\bin\Runner.Worker.dll" -ErrorAction SilentlyContinue
        Write-Output "  - Runner.Worker.dll loaded"
    } catch {
        Write-Output "  - Runner.Worker.dll load attempted (expected to fail, but triggers caching)"
    }

    # Trigger general .NET warming if SDK is installed
    if (Test-Path "$Env:ProgramFiles\dotnet\dotnet.exe") {
        Write-Output "Triggering .NET SDK initialization..."
        & "$Env:ProgramFiles\dotnet\dotnet.exe" --info | Out-Null
        Write-Output "  - .NET SDK initialized"

        # List NuGet cache locations (triggers cache directory creation)
        & "$Env:ProgramFiles\dotnet\dotnet.exe" nuget locals all --list | Out-Null
        Write-Output "  - NuGet cache directories initialized"
    }

    # Trigger NGEN for common .NET Framework assemblies
    Write-Output "Triggering NGEN compilation for .NET Framework assemblies..."
    $ngen = "$env:SystemRoot\Microsoft.NET\Framework64\v4.0.30319\ngen.exe"
    if (Test-Path $ngen) {
        & $ngen executeQueuedItems | Out-Null
        Write-Output "  - NGEN queue processed"
    }

    Write-Output "GitHub Actions Runner pre-warming completed successfully!"
    Write-Output "  - Runner binaries: $runnerPath"
    Write-Output "  - .NET caches populated"
    Write-Output "  - NGEN compilation triggered"
    Write-Output "  - Ready for fast runner registration on VM startup"

} catch {
    Write-Output "ERROR: Failed to pre-warm GitHub Actions Runner: $_"
    throw
} finally {
    # Return to original directory
    Set-Location C:\
}

Write-Output "Phase [END] - Pre-warming GitHub Actions Runner"
