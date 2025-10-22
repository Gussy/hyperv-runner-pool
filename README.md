# Hyper-V Runner Pool

A production-ready pool manager for running GitHub Actions workflows in ephemeral Hyper-V VMs on Windows. Each job runs in a fresh, isolated VM that is automatically destroyed and recreated after completion.

## Features

- **Ephemeral VMs**: Fresh VM for every job - zero state leakage between runs
- **Automatic Runner Cleanup**: Runners auto-remove from GitHub after each job (no stale runners!)
- **Concurrent Execution**: Pool of VMs ready to handle multiple jobs simultaneously
- **GitHub App Authentication**: More secure than PAT tokens - no expiration issues
- **Automatic Lifecycle Management**: VMs are created, registered, and destroyed automatically
- **Serverless Polling**: Monitors VM state locally - no external network required
- **Cross-Platform Development**: Develop and test on macOS, deploy to Windows
- **Production Ready**: Robust error handling, structured logging, and concurrent operations
- **Flexible Images**: Choose between minimal (fast) or enhanced (GitHub-compatible) VM templates
- **Air-Gappable**: Works on isolated networks with no inbound internet access
- **Personal & Org Support**: Works with both personal GitHub accounts and organizations

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         GitHub                              │
│  ┌──────────────┐                                           │
│  │  Repository  │ ◄────── Runners register & pull jobs      │
│  └──────────────┘                                           │
└─────────────────────────────────────────────────────────────┘
                                   ▲
                                   │
                          ┌────────┴─────────┐
                          │   Orchestrator   │
                          │    (Go Binary)   │
                          │                  │
                          │  - VM Pool Mgmt  │
                          │  - State Monitor │
                          │  - GitHub API    │
                          │  - VHDX Inject   │
                          └────────┬─────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                    │                    │
              ▼                    ▼                    ▼
        ┌─────────┐          ┌─────────┐          ┌─────────┐
        │  VM 1   │          │  VM 2   │          │  VM 3   │
        │ Runner  │          │ Runner  │          │ Runner  │
        └─────────┘          └─────────┘          └─────────┘
        (Ephemeral)          (Ephemeral)          (Ephemeral)
         Shutdown             Shutdown             Shutdown
         triggers             triggers             triggers
         recreation           recreation           recreation
```

## Prerequisites

### For Development
- Go 1.21+
- Git
- PowerShell Core (optional, for syntax validation)

### For Production
- Windows 10/11 Pro or Windows Server 2019/2022
- Hyper-V enabled
- Go 1.21+
- Packer
- Git
- 16GB+ RAM (recommended)
- SSD with 40GB+ free space (minimal template) or 100GB+ (enhanced template)

## Quick Start

```powershell
# 1. Clone and Setup
.\bootstrap.ps1
task build

# 2. Create Configuration File
Copy-Item config.example.yaml config.yaml

# 3. Run with Mock VM Manager
.\start.ps1
```

The orchestrator will start with simulated VMs.

### 4. Run Tests

```powershell
task test
```

### 5. Build for Windows

```powershell
# Build Windows binary
task build

# Or build release snapshot with GoReleaser
task release-snapshot
```

## Deployment

### 1: Windows Host Setup

**Quick Bootstrap (Recommended)**

Run the automated bootstrap script to set up everything:

```powershell
# Clone the repository
git clone <your-repo-url>
cd hyperv-runner-pool

# Run bootstrap (as Administrator)
.\bootstrap.ps1
```

The bootstrap script will:
- Install Chocolatey, Go, Packer, Task, GoReleaser, and Git
- Enable Hyper-V (with reboot prompt)
- Create `vms\templates\` and `vms\storage\` directories
- Copy `config.example.yaml` to `config.yaml` for you to edit
- Download Go dependencies

**Manual Setup**

If you prefer manual setup:

1. **Enable Hyper-V**
   ```powershell
   Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All
   # Reboot required
   ```

2. **Install Required Tools**
   ```powershell
   # Install Chocolatey
   Set-ExecutionPolicy Bypass -Scope Process -Force
   iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

   # Install tools
   choco install -y golang packer git go-task goreleaser
   ```

3. **Create Directory Structure**

   Directories are created in the repository by default:
   ```powershell
   New-Item -Path "vms\templates" -ItemType Directory -Force
   New-Item -Path "vms\storage" -ItemType Directory -Force
   ```

   To use a custom location, set these in your `config.yaml` file:
   ```yaml
   hyperv:
     template_path: C:\custom\path\runner-template.vhdx
     storage_path: C:\custom\storage\path
   ```

### 2: Build VM Template

**Choose Your Image Type:**

We provide two Packer configurations:

1. **Minimal** (`runner-basic.pkr.hcl`) - Fast, lightweight
   - Includes: Git, Node.js, Python, .NET, basic tools
   - Best for: Custom workflows, development, cost-sensitive

2. **Enhanced** (`runner-enhanced.pkr.hcl`) - GitHub-compatible
   - Includes: VS Build Tools, multiple language versions, cloud CLIs, databases, etc.
   - Best for: Drop-in GitHub replacement, production
   - Note: Matches GitHub's stated 14GB available space

**Build Minimal Image (Recommended):**

```powershell
cd packer
task init
task build:basic
```

### Phase 3: Create GitHub App

**Create GitHub App:**

1. Go to your organization/personal settings → Developer settings → GitHub Apps → New GitHub App
2. Fill in basic information:
   - **Name**: `runner-pool` (or your choice)
   - **Homepage URL**: Your repository URL
   - **Webhook**: Uncheck "Active" (not needed)
3. Set permissions:
   - For **org-level runners**: `Self-hosted runners` → Read & write
   - For **repo-level runners**: `Administration` → Read & write
4. Click "Create GitHub App"
5. Note your **App ID**
6. Scroll down and click "Generate a private key"
7. Save the downloaded `.pem` file securely

**Install the App:**
- Click "Install App" in the left sidebar
- Choose your organization or personal account
- Select "All repositories" or specific repositories
- Click "Install"

### 4: Configure and Run

**Create configuration file:**

```powershell
# Copy example config
Copy-Item config.example.yaml config.yaml

# Edit with your values
notepad config.yaml
```

**Run the orchestrator:**

```powershell
.\hyperv-runner-pool.exe -c config.yaml
# or
.\start.ps1
```

## How It Works

### Warm Pool Initialization

1. **Orchestrator starts** and creates a warm pool of VMs
2. **For each VM slot**:
   - Generates fresh GitHub runner registration token via GitHub App API
   - Mounts VHDX, injects `runner-config.json` with credentials, unmounts
   - Creates and starts VM
   - VM boots and scheduled task triggers startup script ([configure-runner.ps1](scripts/configure-runner.ps1))
   - VM registers with GitHub as **ephemeral runner**

### Job Execution Cycle

3. **GitHub assigns job** to an available runner
4. **Runner executes job** with `--once` flag ([configure-runner.ps1:169](scripts/configure-runner.ps1#L169)) - single job mode
5. **Job completes**, runner exits
6. **Runner automatically unregisters from GitHub**
7. **VM shuts down** automatically
8. **Orchestrator detects shutdown** via polling every 10s
9. **VM is destroyed** and **recreated** with the **same name** and a fresh token
10. **Cycle repeats** indefinitely

### Complete Ephemeral Runner Lifecycle

1. VM Creation (github-runner-1)
  - Generate registration token via GitHub App
  - Inject runner-config.json into VHDX
  - Create and start VM

2. Runner Registration
  - VM boots, scheduled task runs startup script
  - Runner registers with --ephemeral --once flags
  - Runner appears in GitHub as "Idle"

3. Job Execution
  - GitHub assigns job to runner
  - Runner status: "Idle" → "Active"
  - Job executes in VM

4. Automatic Cleanup
  - Job completes
  - Runner exits (--once flag)
  - GitHub removes runner from UI
  - Runner name "github-runner-1" is now FREE to reuse

5. VM Shutdown
  - Startup script runs: Stop-Computer -Force
  - VM state: "Running" → "Off"

6. Orchestrator Detection
  - MonitorVMState polls every 10 seconds
  - Detects VM state = "Off"
  - Triggers RecreateVM

7. VM Destruction
  - Stop-VM -TurnOff -Force
  - Remove-VM -Force
  - Delete VHDX file

8. VM Recreation (same name: github-runner-1)
  - Generate NEW registration token
  - Create NEW VM with same name
  - Cycle repeats from step 1

## Known Issues Limitations

- **Runner Installation Time**: ~3 minutes per VM due to downloading GitHub Actions runner package (~100-200 MB) during VM startup. This can be eliminated by pre-installing the runner in the Packer template.

---

> This project is not affiliated with, endorsed by, or sponsored by Microsoft Corporation. Hyper-V is a trademark of Microsoft Corporation.
