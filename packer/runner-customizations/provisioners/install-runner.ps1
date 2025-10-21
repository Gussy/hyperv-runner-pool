# Install GitHub Actions Runner
Write-Host "Installing GitHub Actions Runner..."

# Create runner directory
$runnerPath = "C:\actions-runner"
New-Item -Path $runnerPath -ItemType Directory -Force | Out-Null
Set-Location $runnerPath

# Download the latest runner
Write-Host "Downloading GitHub Actions Runner..."
$latestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/actions/runner/releases/latest"
$downloadUrl = $latestRelease.assets | Where-Object { $_.name -like "*win-x64*.zip" } | Select-Object -First 1 -ExpandProperty browser_download_url

Write-Host "Downloading from: $downloadUrl"
Invoke-WebRequest -Uri $downloadUrl -OutFile "actions-runner.zip"

# Extract runner
Write-Host "Extracting runner..."
Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::ExtractToDirectory("$runnerPath\actions-runner.zip", $runnerPath)

# Cleanup zip file
Remove-Item -Path "$runnerPath\actions-runner.zip" -Force

# Verify extraction
if (Test-Path "$runnerPath\config.cmd") {
    Write-Host "GitHub Actions Runner installed successfully to $runnerPath"
} else {
    Write-Error "Failed to install GitHub Actions Runner"
    exit 1
}

# Install Visual C++ Redistributable (required by runner)
Write-Host "Installing Visual C++ Redistributable..."
choco install -y vcredist-all

Write-Host "Runner installation complete!"
