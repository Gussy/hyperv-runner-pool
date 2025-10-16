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
  default = "windows-runner-enhanced"
}

source "hyperv-iso" "windows-runner" {
  vm_name              = "${var.vm_name}"
  iso_url              = "${var.iso_url}"
  iso_checksum         = "${var.iso_checksum}"

  cpus                 = 4
  memory               = 8192 # 8GB (more for enhanced build)
  generation           = 2
  enable_secure_boot   = false

  disk_size            = 76800  # 75GB (provides ~14-20GB free space after OS + software)

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
      "Write-Host '=== Installing Chocolatey ===' -ForegroundColor Cyan",
      "Set-ExecutionPolicy Bypass -Scope Process -Force",
      "[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072",
      "iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))",
      "Write-Host 'Chocolatey installed successfully' -ForegroundColor Green"
    ]
  }

  # Install Visual Studio 2022 Build Tools
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Visual Studio 2022 Build Tools ===' -ForegroundColor Cyan",
      "Write-Host 'This will take 15-30 minutes...'",
      "choco install -y visualstudio2022buildtools --package-parameters \"--add Microsoft.VisualStudio.Workload.VCTools --add Microsoft.VisualStudio.Workload.MSBuildTools --includeRecommended\"",
      "Write-Host 'Visual Studio Build Tools installed' -ForegroundColor Green"
    ]
  }

  # Install Git and version control tools
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Git and VCS tools ===' -ForegroundColor Cyan",
      "choco install -y git",
      "choco install -y git-lfs",
      "choco install -y gh",  # GitHub CLI
      "Write-Host 'Git tools installed' -ForegroundColor Green"
    ]
  }

  # Install multiple Node.js versions via nvm-windows
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Node.js versions ===' -ForegroundColor Cyan",
      "choco install -y nvm",
      "$env:Path = [System.Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path','User')",
      "nvm install 20.11.0",
      "nvm install 18.19.0",
      "nvm install 16.20.2",
      "nvm use 20.11.0",
      "npm install -g yarn pnpm",
      "Write-Host 'Node.js versions installed' -ForegroundColor Green"
    ]
  }

  # Install Python versions
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Python versions ===' -ForegroundColor Cyan",
      "choco install -y python311 --version=3.11.8",
      "choco install -y python310 --version=3.10.13",
      "choco install -y python39 --version=3.9.13",
      "# Add Python to PATH",
      "py -3.11 -m pip install --upgrade pip setuptools wheel",
      "py -3.10 -m pip install --upgrade pip setuptools wheel",
      "py -3.9 -m pip install --upgrade pip setuptools wheel",
      "Write-Host 'Python versions installed' -ForegroundColor Green"
    ]
  }

  # Install .NET SDKs
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing .NET SDKs ===' -ForegroundColor Cyan",
      "choco install -y dotnet-sdk --version=8.0.101",
      "choco install -y dotnet-7.0-sdk",
      "choco install -y dotnet-6.0-sdk",
      "Write-Host '.NET SDKs installed' -ForegroundColor Green"
    ]
  }

  # Install Java/JDK
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Java ===' -ForegroundColor Cyan",
      "choco install -y openjdk17",
      "choco install -y openjdk11",
      "choco install -y maven",
      "choco install -y gradle",
      "Write-Host 'Java tools installed' -ForegroundColor Green"
    ]
  }

  # Install Ruby
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Ruby ===' -ForegroundColor Cyan",
      "choco install -y ruby --version=3.2.3.1",
      "gem install bundler",
      "Write-Host 'Ruby installed' -ForegroundColor Green"
    ]
  }

  # Install Go
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Go ===' -ForegroundColor Cyan",
      "choco install -y golang --version=1.21.6",
      "Write-Host 'Go installed' -ForegroundColor Green"
    ]
  }

  # Install Rust
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Rust ===' -ForegroundColor Cyan",
      "choco install -y rust",
      "Write-Host 'Rust installed' -ForegroundColor Green"
    ]
  }

  # Install PowerShell Core
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing PowerShell Core ===' -ForegroundColor Cyan",
      "choco install -y powershell-core",
      "Write-Host 'PowerShell Core installed' -ForegroundColor Green"
    ]
  }

  # Install Cloud CLIs
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Cloud CLIs ===' -ForegroundColor Cyan",
      "choco install -y awscli",
      "choco install -y azure-cli",
      "choco install -y gcloudsdk",
      "Write-Host 'Cloud CLIs installed' -ForegroundColor Green"
    ]
  }

  # Install Container tools
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Container tools ===' -ForegroundColor Cyan",
      "choco install -y docker-desktop",
      "choco install -y kubernetes-cli",
      "choco install -y kubernetes-helm",
      "Write-Host 'Container tools installed' -ForegroundColor Green"
    ]
  }

  # Install Database clients
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Database clients ===' -ForegroundColor Cyan",
      "choco install -y mysql.workbench",
      "choco install -y postgresql",
      "choco install -y mongodb",
      "choco install -y redis",
      "Write-Host 'Database clients installed' -ForegroundColor Green"
    ]
  }

  # Install Build and utility tools
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Build and utility tools ===' -ForegroundColor Cyan",
      "choco install -y 7zip",
      "choco install -y curl",
      "choco install -y wget",
      "choco install -y jq",
      "choco install -y yq",
      "choco install -y terraform",
      "choco install -y packer",
      "choco install -y cmake",
      "choco install -y ninja",
      "choco install -y nuget.commandline",
      "Write-Host 'Build tools installed' -ForegroundColor Green"
    ]
  }

  # Install GitHub Actions runner
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing GitHub Actions Runner ===' -ForegroundColor Cyan"
    ]
    script = "./scripts/install-runner.ps1"
  }

  # Configure startup script
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Configuring VM startup ===' -ForegroundColor Cyan"
    ]
    script = "./scripts/configure-startup.ps1"
  }

  # Install Visual C++ redistributables
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Visual C++ Redistributables ===' -ForegroundColor Cyan",
      "choco install -y vcredist-all",
      "Write-Host 'VC++ Redistributables installed' -ForegroundColor Green"
    ]
  }

  # Windows Updates (optional but recommended)
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Installing Windows Updates ===' -ForegroundColor Cyan",
      "Write-Host 'This may take 30-60 minutes...'",
      "Install-PackageProvider -Name NuGet -MinimumVersion 2.8.5.201 -Force",
      "Install-Module PSWindowsUpdate -Force",
      "Import-Module PSWindowsUpdate",
      "Get-WindowsUpdate -AcceptAll -Install -AutoReboot:$false",
      "Write-Host 'Windows Updates installed' -ForegroundColor Green"
    ]
  }

  # Final cleanup and optimization
  provisioner "powershell" {
    inline = [
      "Write-Host '=== Final cleanup and optimization ===' -ForegroundColor Cyan",
      "",
      "# Clear temp files",
      "Write-Host 'Clearing temp files...'",
      "Remove-Item -Path $env:TEMP\\* -Recurse -Force -ErrorAction SilentlyContinue",
      "Remove-Item -Path C:\\Windows\\Temp\\* -Recurse -Force -ErrorAction SilentlyContinue",
      "",
      "# Clear package caches",
      "Write-Host 'Clearing package caches...'",
      "choco clean all --confirm",
      "",
      "# Clear Windows update cache",
      "Write-Host 'Clearing Windows update cache...'",
      "Stop-Service wuauserv",
      "Remove-Item -Path C:\\Windows\\SoftwareDistribution\\Download\\* -Recurse -Force -ErrorAction SilentlyContinue",
      "Start-Service wuauserv",
      "",
      "# Optimize disk",
      "Write-Host 'Optimizing disk...'",
      "Optimize-Volume -DriveLetter C -Defrag -Verbose",
      "",
      "# Compact OS (optional - saves ~2GB)",
      "# Compact.exe /CompactOS:always",
      "",
      "Write-Host '=== Template preparation complete! ===' -ForegroundColor Green",
      "Write-Host 'Installed software summary:' -ForegroundColor Cyan",
      "Write-Host '- Visual Studio 2022 Build Tools'",
      "Write-Host '- Git, Git LFS, GitHub CLI'",
      "Write-Host '- Node.js (16, 18, 20) + npm, yarn, pnpm'",
      "Write-Host '- Python (3.9, 3.10, 3.11)'",
      "Write-Host '- .NET SDK (6, 7, 8)'",
      "Write-Host '- Java (11, 17) + Maven, Gradle'",
      "Write-Host '- Ruby 3.2 + Bundler'",
      "Write-Host '- Go 1.21'",
      "Write-Host '- Rust'",
      "Write-Host '- PowerShell Core'",
      "Write-Host '- AWS CLI, Azure CLI, gcloud'",
      "Write-Host '- Docker Desktop, kubectl, Helm'",
      "Write-Host '- Database clients (MySQL, PostgreSQL, MongoDB, Redis)'",
      "Write-Host '- Build tools (CMake, Ninja, Terraform, Packer)'",
      "Write-Host '- GitHub Actions Runner'",
      ""
    ]
  }
}
