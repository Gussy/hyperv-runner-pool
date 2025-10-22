# GitHub Actions Runner - Shared Base Configuration
# This file contains common configuration used by both basic and enhanced builds
# Variables are defined in variables.pkr.hcl

# Common locals for all runner builds
locals {
  # Elevated credentials used by all provisioners
  elevated_user     = "Administrator"
  elevated_password = "password"

  # Common source configuration
  runner_source_defaults = {
    boot_command          = ["a<enter><wait>a<enter><wait>a<enter><wait>a<enter>"]
    boot_wait             = "1s"
    communicator          = "winrm"
    enable_dynamic_memory = true
    enable_secure_boot    = false
    generation            = 2
    guest_additions_mode  = "disable"
    shutdown_timeout      = "30m"
    skip_export           = false
    temp_path             = "."
    winrm_password        = "password"
    winrm_username        = "Administrator"
  }

  # Common sysprep commands
  sysprep_commands = [
    "Write-Output 'Phase-5-Deprovisioning'",
    "if (!(Test-Path -Path $Env:SystemRoot\\system32\\Sysprep\\unattend.xml)){ Write-Output 'No file';exit (10)}",
    "& $Env:SystemRoot\\System32\\Sysprep\\Sysprep.exe /oobe /generalize /shutdown /quiet /unattend:C:\\Windows\\system32\\ssysprep\\unattend.xml"
  ]
}
