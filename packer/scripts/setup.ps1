# Setup script for Packer - Enables WinRM for provisioning
Write-Host "Setting up WinRM for Packer..."

# Enable WinRM
Write-Host "Enabling WinRM..."
Enable-PSRemoting -Force -SkipNetworkProfileCheck

# Configure WinRM
Write-Host "Configuring WinRM..."
winrm quickconfig -q
winrm quickconfig -transport:http
winrm set winrm/config '@{MaxTimeoutms="1800000"}'
winrm set winrm/config/winrs '@{MaxMemoryPerShellMB="1024"}'
winrm set winrm/config/service '@{AllowUnencrypted="true"}'
winrm set winrm/config/service/auth '@{Basic="true"}'
winrm set winrm/config/client/auth '@{Basic="true"}'

# Configure firewall
Write-Host "Configuring Windows Firewall for WinRM..."
netsh advfirewall firewall set rule group="Windows Remote Management" new enable=yes

# Restart WinRM service
Write-Host "Restarting WinRM service..."
Restart-Service winrm

# Set network profile to private
Write-Host "Setting network profile to private..."
Set-NetConnectionProfile -NetworkCategory Private -ErrorAction SilentlyContinue

# Disable Windows Defender real-time protection for faster builds
Write-Host "Adjusting Windows Defender settings..."
Set-MpPreference -DisableRealtimeMonitoring $true -ErrorAction SilentlyContinue

# Set execution policy
Write-Host "Setting execution policy..."
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Force

Write-Host "WinRM setup complete!"
