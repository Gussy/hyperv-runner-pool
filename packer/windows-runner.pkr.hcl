packer {
  required_plugins {
    hyperv = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/hyperv"
    }
  }
}

variable "iso_url" {
  type    = string
  default = "https://software-download.microsoft.com/download/sg/20348.169.210806-2348.fe_release_svc_refresh_SERVER_EVAL_x64FRE_en-us.iso"
}

variable "iso_checksum" {
  type    = string
  default = "sha256:3e4fa6d8507b554856fc9ca6079cc402df11a8b79344871669f0251535255325"
}

variable "vm_name" {
  type    = string
  default = "windows-runner"
}

source "hyperv-iso" "windows-runner" {
  vm_name              = "${var.vm_name}"
  iso_url              = "${var.iso_url}"
  iso_checksum         = "${var.iso_checksum}"

  cpus                 = 4
  memory               = 6144 # 6GB
  generation           = 2
  enable_secure_boot   = false

  disk_size            = 30720  # 30GB (provides ~14-16GB free space after OS + tools)

  communicator         = "winrm"
  winrm_username       = "Administrator"
  winrm_password       = "PackerPassword123!"
  winrm_timeout        = "12h"

  shutdown_command     = "shutdown /s /t 10 /f /d p:4:1 /c \"Packer Shutdown\""
  shutdown_timeout     = "15m"

  cd_files = [
    "./autounattend.xml",
    "./scripts/setup.ps1"
  ]

  cd_label = "packer"

  output_directory = "output-${var.vm_name}"
}

build {
  sources = ["source.hyperv-iso.windows-runner"]

  # Wait for initial setup to complete
  provisioner "powershell" {
    inline = [
      "Write-Host 'Waiting for system to stabilize...'",
      "Start-Sleep -Seconds 30"
    ]
  }

  # Install Chocolatey
  provisioner "powershell" {
    inline = [
      "Write-Host 'Installing Chocolatey...'",
      "Set-ExecutionPolicy Bypass -Scope Process -Force",
      "[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072",
      "iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))"
    ]
  }

  # Install common tools
  provisioner "powershell" {
    inline = [
      "Write-Host 'Installing common tools...'",
      "choco install -y git",
      "choco install -y nodejs-lts",
      "choco install -y python",
      "choco install -y dotnet-sdk",
      "choco install -y 7zip",
      "choco install -y curl",
      "choco install -y wget"
    ]
  }

  # Install GitHub Actions runner
  provisioner "powershell" {
    script = "./scripts/install-runner.ps1"
  }

  # Configure startup script
  provisioner "powershell" {
    script = "./scripts/configure-startup.ps1"
  }

  # Windows updates and cleanup
  provisioner "powershell" {
    inline = [
      "Write-Host 'Cleaning up...'",
      "# Clear temp files",
      "Remove-Item -Path $env:TEMP\\* -Recurse -Force -ErrorAction SilentlyContinue",
      "Remove-Item -Path C:\\Windows\\Temp\\* -Recurse -Force -ErrorAction SilentlyContinue",
      "# Clear package caches",
      "choco clean all --confirm",
      "# Optimize disk",
      "Optimize-Volume -DriveLetter C -Defrag -Verbose",
      "Write-Host 'Template preparation complete!'"
    ]
  }
}
