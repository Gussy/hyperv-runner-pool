# Install basic development tools
Write-Output "Phase [START] - Basic Development Tools"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# Install Git
choco install git -y --no-progress --limit-output
if (Test-Path ("$Env:ProgramFiles\Git")) {
  Write-Output "Git installed successfully"
  [Environment]::SetEnvironmentVariable("Path",[Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + ";$Env:ProgramFiles\Git\cmd",[EnvironmentVariableTarget]::Machine)
}

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

Write-Output "Phase [END] - Basic Development Tools"
