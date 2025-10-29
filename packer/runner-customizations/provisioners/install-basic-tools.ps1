# Install basic development tools
Write-Output "Phase [START] - Basic Development Tools"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# Install Git (includes bash via Git Bash)
choco install git -y --no-progress --limit-output
if (Test-Path ("$Env:ProgramFiles\Git")) {
  Write-Output "Git installed successfully"
  # Add Git cmd, usr/bin (for bash, ssh, etc.), and bin to PATH
  $gitPaths = ";$Env:ProgramFiles\Git\cmd;$Env:ProgramFiles\Git\usr\bin;$Env:ProgramFiles\Git\bin"
  [Environment]::SetEnvironmentVariable("Path",[Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + $gitPaths,[EnvironmentVariableTarget]::Machine)
  Write-Output "Bash and Git utilities added to PATH"
}

# Install Git LFS
choco install git-lfs -y --no-progress --limit-output
Write-Output "Git LFS installed successfully"

# Install Node.js LTS
choco install nodejs-lts -y --no-progress --limit-output
if (Test-Path ("$Env:ProgramFiles\nodejs")) {
  Write-Output "Node.js LTS installed successfully"
}

# Install Python
choco install python -y --no-progress --limit-output
if (Test-Path ("$Env:ProgramData\chocolatey\lib\python")) {
  Write-Output "Python installed successfully"
}

# Install .NET SDK
choco install dotnet-sdk -y --no-progress --limit-output
if (Test-Path ("$Env:ProgramFiles\dotnet")) {
  Write-Output ".NET SDK installed successfully"
}

# Install 7-Zip
choco install 7zip -y --no-progress --limit-output
if (Test-Path ("$Env:ProgramFiles\7-Zip")) {
  Write-Output "7-Zip installed successfully"
}

# Install curl
choco install curl -y --no-progress --limit-output
Write-Output "curl installed successfully"

# Install wget
choco install wget -y --no-progress --limit-output
Write-Output "wget installed successfully"

# Install jq (JSON processor)
choco install jq -y --no-progress --limit-output
Write-Output "jq installed successfully"

# Install OpenSSL
choco install openssl -y --no-progress --limit-output
Write-Output "OpenSSL installed successfully"

# Install CMake (build system generator)
choco install cmake -y --no-progress --limit-output
if (Test-Path ("$Env:ProgramFiles\CMake")) {
  Write-Output "CMake installed successfully"
}

# Install Ninja (build system)
choco install ninja -y --no-progress --limit-output
Write-Output "Ninja installed successfully"

Write-Output "Phase [END] - Basic Development Tools"
