# Install Enhanced Development Toolchain
# This script installs the full suite of development tools for GitHub Actions runners

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Enhanced Development Toolchain Installer" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Install Visual Studio 2022 Build Tools
Write-Host '=== Installing Visual Studio 2022 Build Tools ===' -ForegroundColor Cyan
Write-Host 'This will take 15-30 minutes...'
choco install -y visualstudio2022buildtools --package-parameters "--add Microsoft.VisualStudio.Workload.VCTools --add Microsoft.VisualStudio.Workload.MSBuildTools --includeRecommended"
Write-Host 'Visual Studio Build Tools installed' -ForegroundColor Green

# Install Git and version control tools (includes bash)
Write-Host '=== Installing Git and VCS tools ===' -ForegroundColor Cyan
choco install -y git
# Add Git bash and utilities to PATH
if (Test-Path ("$Env:ProgramFiles\Git")) {
  $gitPaths = ";$Env:ProgramFiles\Git\cmd;$Env:ProgramFiles\Git\usr\bin;$Env:ProgramFiles\Git\bin"
  [Environment]::SetEnvironmentVariable("Path",[Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + $gitPaths,[EnvironmentVariableTarget]::Machine)
  Write-Host 'Bash and Git utilities added to PATH' -ForegroundColor Green
}
choco install -y git-lfs
choco install -y gh  # GitHub CLI
Write-Host 'Git tools installed' -ForegroundColor Green

# Install multiple Node.js versions via nvm-windows
Write-Host '=== Installing Node.js versions ===' -ForegroundColor Cyan
choco install -y nvm
$env:Path = [System.Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path','User')
nvm install 20.11.0
nvm install 18.19.0
nvm install 16.20.2
nvm use 20.11.0
npm install -g yarn pnpm
Write-Host 'Node.js versions installed' -ForegroundColor Green

# Install Python versions
Write-Host '=== Installing Python versions ===' -ForegroundColor Cyan
choco install -y python311 --version=3.11.8
choco install -y python310 --version=3.10.13
choco install -y python39 --version=3.9.13
py -3.11 -m pip install --upgrade pip setuptools wheel
py -3.10 -m pip install --upgrade pip setuptools wheel
py -3.9 -m pip install --upgrade pip setuptools wheel
Write-Host 'Python versions installed' -ForegroundColor Green

# Install .NET SDKs
Write-Host '=== Installing .NET SDKs ===' -ForegroundColor Cyan
choco install -y dotnet-sdk --version=8.0.101
choco install -y dotnet-7.0-sdk
choco install -y dotnet-6.0-sdk
Write-Host '.NET SDKs installed' -ForegroundColor Green

# Install Java/JDK
Write-Host '=== Installing Java ===' -ForegroundColor Cyan
choco install -y openjdk17
choco install -y openjdk11
choco install -y maven
choco install -y gradle
Write-Host 'Java tools installed' -ForegroundColor Green

# Install Ruby
Write-Host '=== Installing Ruby ===' -ForegroundColor Cyan
choco install -y ruby --version=3.2.3.1
gem install bundler
Write-Host 'Ruby installed' -ForegroundColor Green

# Install Go
Write-Host '=== Installing Go ===' -ForegroundColor Cyan
choco install -y golang --version=1.21.6
Write-Host 'Go installed' -ForegroundColor Green

# Install Rust
Write-Host '=== Installing Rust ===' -ForegroundColor Cyan
choco install -y rust
Write-Host 'Rust installed' -ForegroundColor Green

# Install PowerShell Core
Write-Host '=== Installing PowerShell Core ===' -ForegroundColor Cyan
choco install -y powershell-core
Write-Host 'PowerShell Core installed' -ForegroundColor Green

# Install Cloud CLIs
Write-Host '=== Installing Cloud CLIs ===' -ForegroundColor Cyan
choco install -y awscli
choco install -y azure-cli
choco install -y gcloudsdk
Write-Host 'Cloud CLIs installed' -ForegroundColor Green

# Install Container tools
Write-Host '=== Installing Container tools ===' -ForegroundColor Cyan
choco install -y docker-desktop
choco install -y kubernetes-cli
choco install -y kubernetes-helm
Write-Host 'Container tools installed' -ForegroundColor Green

# Install Database clients
Write-Host '=== Installing Database clients ===' -ForegroundColor Cyan
choco install -y mysql.workbench
choco install -y postgresql
choco install -y mongodb
choco install -y redis
Write-Host 'Database clients installed' -ForegroundColor Green

# Install Build and utility tools
Write-Host '=== Installing Build and utility tools ===' -ForegroundColor Cyan
choco install -y 7zip
choco install -y curl
choco install -y wget
choco install -y jq
choco install -y yq
choco install -y openssl
choco install -y terraform
choco install -y packer
choco install -y cmake
choco install -y ninja
choco install -y nuget.commandline
Write-Host 'Build tools installed' -ForegroundColor Green

# Install Visual C++ redistributables
Write-Host '=== Installing Visual C++ Redistributables ===' -ForegroundColor Cyan
choco install -y vcredist-all
Write-Host 'VC++ Redistributables installed' -ForegroundColor Green

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "Enhanced Toolchain Installation Complete" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
