# GitHub Actions Runner - Enhanced Build
# Extends hv-packer's Windows Server 2022 base with full development toolchain
# Variables are defined in variables.pkr.hcl

# Use the same source configuration as hv-packer, with shared defaults from runner-base.pkr.hcl
source "hyperv-iso" "runner-enhanced" {
  boot_command          = local.runner_source_defaults.boot_command
  boot_wait             = local.runner_source_defaults.boot_wait
  communicator          = local.runner_source_defaults.communicator
  cpus                  = var.cpus
  disk_size             = var.disk_size
  enable_dynamic_memory = local.runner_source_defaults.enable_dynamic_memory
  enable_secure_boot    = local.runner_source_defaults.enable_secure_boot
  generation            = local.runner_source_defaults.generation
  guest_additions_mode  = local.runner_source_defaults.guest_additions_mode
  iso_checksum          = "${var.iso_checksum_type}:${var.iso_checksum}"
  iso_url               = var.iso_url
  memory                = var.memory
  output_directory      = var.output_directory
  secondary_iso_images  = [var.secondary_iso_image]
  shutdown_timeout      = local.runner_source_defaults.shutdown_timeout
  skip_export           = local.runner_source_defaults.skip_export
  switch_name           = var.switch_name
  temp_path             = local.runner_source_defaults.temp_path
  vlan_id               = var.vlan_id
  vm_name               = var.vm_name
  winrm_password        = local.runner_source_defaults.winrm_password
  winrm_timeout         = "12h"  # Longer timeout for enhanced build
  winrm_username        = local.runner_source_defaults.winrm_username
}

build {
  name = "runner-enhanced"
  sources = ["source.hyperv-iso.runner-enhanced"]

  # Phase 1: Initial system setup (from hv-packer)
  provisioner "powershell" {
    elevated_password = local.elevated_password
    elevated_user     = local.elevated_user
    script            = "./hv-packer/extra/scripts/windows/shared/phase-1.ps1"
  }

  provisioner "windows-restart" {
    restart_timeout = "1h"
  }

  # Phase 2: System configuration (from hv-packer)
  provisioner "powershell" {
    elevated_password = local.elevated_password
    elevated_user     = local.elevated_user
    script            = "./hv-packer/extra/scripts/windows/shared/phase-2.ps1"
  }

  provisioner "windows-restart" {
    pause_before          = "1m0s"
    restart_check_command = "powershell -command \"& {Write-Output 'restarted.'}\""
    restart_timeout       = "2h"
  }

  # Windows Updates - First pass
  provisioner "windows-update" {
    search_criteria = "IsInstalled=0"
    update_limit    = 10
  }

  provisioner "windows-restart" {
    restart_timeout = "1h"
  }

  # Windows Updates - Second pass
  provisioner "windows-update" {
    search_criteria = "IsInstalled=0"
    update_limit    = 10
  }

  provisioner "windows-restart" {
    restart_timeout = "1h"
  }

  # CUSTOM: Install enhanced development toolchain
  provisioner "powershell" {
    elevated_password = local.elevated_password
    elevated_user     = local.elevated_user
    script            = "./runner-customizations/provisioners/install-enhanced-tools.ps1"
  }


  # Sysprep configuration
  provisioner "file" {
    destination = "C:\\Windows\\System32\\Sysprep\\unattend.xml"
    source      = var.sysprep_unattended
  }

  # Sysprep and shutdown
  provisioner "powershell" {
    inline = local.sysprep_commands
  }
}
