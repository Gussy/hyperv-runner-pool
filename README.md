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

### For Development (macOS)
- Go 1.21+
- Git
- PowerShell Core (optional, for syntax validation)

### For Production (Windows)
- Windows 10/11 Pro or Windows Server 2019/2022
- Hyper-V enabled
- Go 1.21+
- Packer
- Git
- 16GB+ RAM (recommended)
- SSD with 40GB+ free space (minimal template) or 100GB+ (enhanced template)

## Quick Start (macOS Development)

### 1. Clone and Setup

```bash
git clone <your-repo-url>
cd hyperv-runner-pool

# Dependencies are already installed via go.mod
go mod download
```

### 2. Create Configuration File

```bash
cp config.example.yaml config.yaml
# Edit config.yaml and set:
#   - github.app_id (your GitHub App ID)
#   - github.app_private_key_path (path to your .pem file)
#   - github.org or github.user (organization or username)
#   - github.repo (repository name, or leave empty for org-level runners)
#   - debug.use_mock: true (for macOS testing)
```

Example config for development:

```yaml
github:
  app_id: 123456
  app_private_key_path: /path/to/github-app.pem
  org: my-org
  repo: my-repo
runners:
  pool_size: 1
  name_prefix: "runner-"
debug:
  use_mock: true      # Enable mock mode for macOS
  log_level: debug
  log_format: text
```

### 3. Run with Mock VM Manager

```bash
# Run with config file
go run ./cmd/hyperv-runner-pool -c config.yaml
```

The orchestrator will start with simulated VMs. Perfect for development!

### 4. Run Tests

```bash
go test -v ./...

# Or using Task
task test
```

### 5. Build for Windows

**Option 1: Task (Recommended)**
```bash
# Build Windows binary
task build

# Or build release snapshot with GoReleaser
task release-snapshot
```

**Option 2: Manual Build**
```bash
GOOS=windows GOARCH=amd64 go build -o hyperv-runner-pool.exe ./cmd/hyperv-runner-pool
```

**Option 3: GoReleaser**
```bash
# Install GoReleaser (macOS)
brew install goreleaser

# Build snapshot (local testing)
goreleaser build --snapshot --clean --single-target

# The binary will be in dist/hyperv-runner-pool_windows_amd64_v1/
```

## Building and Releasing

### Creating a Release

This project uses [GoReleaser](https://goreleaser.com/) for building and releasing Windows binaries.

**Automated Release (via GitHub Actions):**
```bash
# Create and push a version tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GitHub Actions will automatically:
# 1. Build the Windows binary
# 2. Create a GitHub release
# 3. Upload the binary and archives
# 4. Generate changelog
```

**Manual Release:**
```bash
# Ensure you're on the commit you want to release
git tag -a v1.0.0 -m "Release v1.0.0"

# Set GitHub token
export GITHUB_TOKEN="your-github-token"

# Run GoReleaser
goreleaser release --clean
```

**Local Build (testing):**
```bash
# Build without releasing
goreleaser build --snapshot --clean

# Output will be in dist/ directory
```

### What Gets Released

Each release includes:
- `hyperv-runner-pool.exe` - Windows binary
- `config.example.yaml` - Configuration template
- `start.ps1` - Startup script (if available)
- `README.md` - Documentation
- `packer/` - Complete Packer configuration
- `scripts/` - PowerShell scripts
- `checksums.txt` - SHA256 checksums

### Downloading Releases

Users can download pre-built releases from the GitHub Releases page instead of building from source:

1. Go to the [Releases](../../releases) page
2. Download the latest `hyperv-runner-pool_*_windows_amd64.zip`
3. Extract and follow the README included in the archive

## Production Deployment (Windows)

### Phase 1: Windows Setup

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

### Phase 2: Build VM Template

**Choose Your Image Type:**

We provide two Packer configurations:

1. **Minimal** (`windows-runner.pkr.hcl`) - Fast, lightweight
   - Build time: 30-45 minutes
   - Total disk: 30GB (~14GB free after OS + tools)
   - Includes: Git, Node.js, Python, .NET, basic tools
   - Best for: Custom workflows, development, cost-sensitive

2. **Enhanced** (`windows-runner-enhanced.pkr.hcl`) - GitHub-compatible
   - Build time: 2-4 hours
   - Total disk: 75GB (~14-20GB free after OS + software)
   - Includes: VS Build Tools, multiple language versions, cloud CLIs, databases, etc.
   - Best for: Drop-in GitHub replacement, production
   - Note: Matches GitHub's stated 14GB available space

**See `.notes/packer-image-comparison.md` for detailed comparison**

**Build Minimal Image (Recommended for first-time):**
```powershell
cd packer
packer init .
packer build windows-runner.pkr.hcl

# Copy template to local vms directory (after ~45 minute build)
Copy-Item ".\output-windows-runner\Virtual Hard Disks\*.vhdx" `
          "..\vms\templates\runner-template.vhdx"
```

**Or Build Enhanced Image:**
```powershell
cd packer
packer init .
packer build windows-runner-enhanced.pkr.hcl

# Copy template to local vms directory (after ~3 hour build)
Copy-Item ".\output-windows-runner-enhanced\Virtual Hard Disks\*.vhdx" `
          "..\vms\templates\runner-template.vhdx"
```

**Using Custom Path:**

If you set `VM_TEMPLATE_PATH` in your `.env`, copy to that location instead:
```powershell
Copy-Item ".\output-windows-runner\Virtual Hard Disks\*.vhdx" `
          "C:\your\custom\path\runner-template.vhdx"
```

### Phase 3: Create GitHub App

**Why GitHub Apps?**
- More secure than Personal Access Tokens (PAT)
- Tokens are short-lived and auto-refreshed
- No token expiration issues
- Fine-grained permissions

**Create GitHub App:**

1. Go to your organization/personal settings → Developer settings → GitHub Apps → New GitHub App
2. Fill in basic information:
   - **Name**: `my-runner-pool` (or your choice)
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

### Phase 4: Configure and Run

**Create configuration file:**

```powershell
# Copy example config
Copy-Item config.example.yaml config.yaml

# Edit with your values
notepad config.yaml
```

**Example production config:**

```yaml
github:
  app_id: 123456                                         # Your GitHub App ID
  app_private_key_path: C:\keys\my-app.private-key.pem  # Path to downloaded .pem
  org: my-organization                                   # Your org or username
  repo: my-repo                                          # Optional for orgs, required for users

runners:
  pool_size: 4
  name_prefix: "runner-"

hyperv:
  template_path: C:\vms\templates\runner-template.vhdx
  storage_path: C:\vms\storage
  vm_username: "Administrator"
  vm_password: "vagrant"

debug:
  log_level: info
  log_format: text
```

**Run the orchestrator:**

```powershell
# Option 1: Direct execution
.\hyperv-runner-pool.exe -c config.yaml

# Option 2: Using start script (if available)
.\start.ps1
```

## Configuration Reference

All configuration is done via a YAML file (default: `config.yaml`). See [config.example.yaml](config.example.yaml) for a complete example.

### GitHub Configuration

```yaml
github:
  # GitHub App authentication (more secure than PAT)
  app_id: 123456
  app_private_key_path: C:\path\to\github-app-private-key.pem

  # Account configuration - use EITHER org OR user
  org: your-organization     # For organizations
  # user: your-username      # For personal accounts (alternative to org)

  # Repository (optional for orgs, required for personal accounts)
  repo: your-repository      # Leave empty for org-level runners
```

**Account Types:**
- **Organizations**: Can use org-level runners (leave `repo` empty) or repo-level runners (specify `repo`)
- **Personal Accounts**: Must specify a repository - GitHub doesn't support account-level runners for users

**Required GitHub App Permissions:**
- Organization/Account-level runners: **Self-hosted runners** (Read & write)
- Repository-level runners: **Administration** (Read & write)

### Runner Configuration

```yaml
runners:
  pool_size: 4                    # Number of concurrent VMs
  name_prefix: "runner-"          # VM names: runner-1, runner-2, etc.
  labels: ["custom", "label"]     # Optional: additional labels beyond defaults
  runner_group: "default"         # Optional: runner group (org-level only)
```

### Hyper-V Configuration

```yaml
hyperv:
  template_path: C:\path\to\template.vhdx    # Path to VM template
  storage_path: C:\vm_storage                # Where to store VM disks
  vm_username: "Administrator"               # PowerShell Direct username
  vm_password: "YourPassword"                # PowerShell Direct password
```

### Debug Configuration

```yaml
debug:
  use_mock: false          # Use mock VMs for testing (macOS/Linux)
  log_level: info         # debug, info, warn, error
  log_format: text        # text or json
```

### Complete Example

```yaml
github:
  app_id: 2150342
  app_private_key_path: C:\keys\my-app.pem
  org: acme-corp
  repo: my-repo
runners:
  pool_size: 4
  name_prefix: "gh-runner-"
  labels: ["windows", "x64"]
hyperv:
  template_path: C:\vms\templates\runner-template.vhdx
  storage_path: C:\vms\storage
  vm_username: "Administrator"
  vm_password: "SecurePassword123"
debug:
  log_level: info
  log_format: text
```

## How It Works

### Warm Pool Initialization

1. **Orchestrator starts** ([main.go:1168-1174](main.go#L1168-L1174)) and creates a warm pool of VMs
2. **For each VM slot**:
   - Generates fresh GitHub runner registration token via GitHub App API ([main.go:793-906](main.go#L793-L906))
   - Mounts VHDX, injects `runner-config.json` with credentials, unmounts ([main.go:248-388](main.go#L248-L388))
   - Creates and starts VM ([main.go:144-206](main.go#L144-L206))
   - VM boots and scheduled task triggers startup script ([configure-runner.ps1](scripts/configure-runner.ps1))
   - VM registers with GitHub as **ephemeral runner** ([configure-runner.ps1:147](scripts/configure-runner.ps1#L147))

### Job Execution Cycle

3. **GitHub assigns job** to an available runner
4. **Runner executes job** with `--once` flag ([configure-runner.ps1:169](scripts/configure-runner.ps1#L169)) - single job mode
5. **Job completes**, runner exits
6. **Runner automatically unregisters from GitHub** - this is the magic of the `--ephemeral` flag!
7. **VM shuts down** automatically ([configure-runner.ps1:175](scripts/configure-runner.ps1#L175))
8. **Orchestrator detects shutdown** via polling every 10s ([main.go:948-978](main.go#L948-L978))
9. **VM is destroyed** ([main.go:218-235](main.go#L218-L235)) and **recreated** ([main.go:907-942](main.go#L907-L942)) with the **same name** and a fresh token
10. **Cycle repeats** indefinitely

**No webhooks, no inbound network access required** - the orchestrator polls VM state locally.

## Ephemeral Runner Lifecycle

### The Magic of `--ephemeral`

This system uses GitHub's ephemeral runner feature to automatically clean up runners after each job. This is why VMs can be recreated with the same name without conflicts!

### How Ephemeral Runners Work

When a runner is registered with the `--ephemeral` flag ([configure-runner.ps1:147](scripts/configure-runner.ps1#L147)):

```powershell
$configArgs += @(
    "--token", $config.token,
    "--name", $config.name,
    "--labels", $config.labels,
    "--ephemeral",      # ← Automatic cleanup after ONE job
    "--disableupdate"
)
```

**GitHub automatically unregisters the runner after it completes a single job.** This means:

1. ✅ **No manual cleanup needed** - GitHub handles runner removal
2. ✅ **No name conflicts** - Old runner is gone before new VM registers
3. ✅ **No stale runners** accumulating in GitHub settings
4. ✅ **Perfect for ephemeral VMs** - matches the disposable nature

### Complete Lifecycle with Code References

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. VM Creation (github-runner-1)                                │
│    - Generate registration token via GitHub App                 │
│      → main.go:793-906 (getGitHubRunnerToken)                   │
│    - Inject runner-config.json into VHDX                        │
│      → main.go:248-388 (InjectConfig)                           │
│    - Create and start VM                                        │
│      → main.go:144-206 (CreateVM)                               │
└─────────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ 2. Runner Registration                                           │
│    - VM boots, scheduled task runs startup script               │
│      → scripts/configure-runner.ps1                             │
│    - Runner registers with --ephemeral --once flags             │
│      → configure-runner.ps1:147, 169                            │
│    - Runner appears in GitHub as "Idle"                         │
└─────────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ 3. Job Execution                                                 │
│    - GitHub assigns job to runner                               │
│    - Runner status: "Idle" → "Active"                           │
│    - Job executes in VM                                         │
└─────────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ 4. Automatic Cleanup (--ephemeral does this!)                   │
│    - Job completes                                              │
│    - Runner exits (--once flag)                                 │
│    - ✨ GitHub AUTOMATICALLY removes runner from UI             │
│    - Runner name "github-runner-1" is now FREE to reuse!        │
└─────────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ 5. VM Shutdown                                                   │
│    - Startup script runs: Stop-Computer -Force                  │
│      → configure-runner.ps1:175                                 │
│    - VM state: "Running" → "Off"                                │
└─────────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ 6. Orchestrator Detection                                        │
│    - MonitorVMState polls every 10 seconds                      │
│      → main.go:948-978                                          │
│    - Detects VM state = "Off"                                   │
│    - Triggers RecreateVM                                        │
│      → main.go:907-942                                          │
└─────────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ 7. VM Destruction                                                │
│    - Stop-VM -TurnOff -Force                                    │
│    - Remove-VM -Force                                           │
│    - Delete VHDX file                                           │
│      → main.go:218-235 (DestroyVM)                              │
└─────────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────────┐
│ 8. VM Recreation (SAME NAME: github-runner-1)                   │
│    - Generate NEW registration token                            │
│    - Create NEW VM with same name (no conflict!)               │
│    - Cycle repeats from step 1                                  │
│      → main.go:828-842 (createAndRegisterVM)                    │
└─────────────────────────────────────────────────────────────────┘
```

### Why This Works Without Name Conflicts

**Problem:** If runners weren't removed, registering a new runner with the same name would fail.

**Solution:** The `--ephemeral` flag ensures the old `github-runner-1` is automatically removed from GitHub before the new `github-runner-1` VM is created. This happens **automatically** - no API calls needed!

### Key Flags Explained

| Flag | Purpose | Location |
|------|---------|----------|
| `--ephemeral` | Auto-remove runner from GitHub after ONE job | [configure-runner.ps1:147](scripts/configure-runner.ps1#L147) |
| `--once` | Stop runner after executing ONE job | [configure-runner.ps1:169](scripts/configure-runner.ps1#L169) |
| `--disableupdate` | Prevent runner from self-updating (immutable VMs) | [configure-runner.ps1:148](scripts/configure-runner.ps1#L148) |

### Labels Include `ephemeral`

The runner is also tagged with the `ephemeral` label ([main.go:166](main.go#L166)):

```go
defaultLabels := []string{"self-hosted", "Windows", "X64", "ephemeral"}
```

This allows workflows to specifically target ephemeral runners if needed:

```yaml
runs-on: [self-hosted, ephemeral]
```

## Development Workflow

### Task Commands (Recommended)

This project uses [Task](https://taskfile.dev) for common development workflows:

```bash
# Show all available tasks
task --list

# Common tasks
task build          # Build Windows binary
task test           # Run all tests
task run            # Run with mock VMs
task clean          # Clean build artifacts
task fmt            # Format code
task lint           # Run linter

# See all tasks
task
```

### macOS Development Loop

```bash
# 1. Make code changes
vim pkg/orchestrator/orchestrator.go

# 2. Run tests
go test -v ./...

# 3. Test with mock VMs
go run ./cmd/hyperv-runner-pool -c config.yaml

# 4. Build for Windows
GOOS=windows GOARCH=amd64 go build -o hyperv-runner-pool.exe ./cmd/hyperv-runner-pool

# 5. Transfer to Windows machine
scp hyperv-runner-pool.exe user@windows-machine:C:/runner/
```

### Testing on Windows

```powershell
# Stop existing runner pool
# Ctrl+C or kill process

# Replace binary
Copy-Item hyperv-runner-pool.exe C:\runner\

# Restart
.\start.ps1
```

## Troubleshooting

### VMs Not Appearing in GitHub

1. Check orchestrator logs for errors
2. Verify GitHub PAT has correct permissions
3. Ensure Windows machine has internet access to reach GitHub API
4. Check VM startup logs for runner registration errors

### VM Creation Fails

1. Verify Hyper-V is enabled: `Get-VM`
2. Check template exists: `Test-Path vms\templates\runner-template.vhdx`
3. Ensure sufficient disk space (40GB+ for minimal, 100GB+ for enhanced)
4. Check orchestrator logs for PowerShell errors
5. Verify paths in startup logs or check your `.env` for custom paths
6. Ensure VHDX is not mounted elsewhere (prevents VM creation)

### VMs Not Recreating After Jobs

1. Check monitoring goroutine logs
2. Verify VM is actually shutting down after job completion
3. Check `MonitorVMState` polling interval (default: 10s)
4. Ensure orchestrator has permissions to query Hyper-V state

### Performance Issues

1. Increase VM resources (CPU, memory) in main.go HyperVManager
2. Reduce pool size if system is overloaded
3. Monitor disk I/O (differencing disks are already used by default)
4. Adjust polling interval if recreation is too slow
5. Ensure template VHDX is on fast storage (SSD recommended)

## Project Structure

```
.
├── cmd/                             # Command-line applications
│   └── hyperv-runner-pool/         # Main CLI application
│       └── main.go                 # Application entry point
├── pkg/                             # Reusable packages
│   ├── config/                     # Configuration management
│   │   └── config.go
│   ├── github/                     # GitHub API client
│   │   └── client.go
│   ├── logger/                     # Logging setup
│   │   └── logger.go
│   ├── orchestrator/               # VM pool orchestration
│   │   ├── orchestrator.go
│   │   ├── monitoring.go
│   │   └── orchestrator_test.go
│   └── vmmanager/                  # VM management
│       ├── manager.go              # Interface and types
│       ├── hyperv.go               # Hyper-V implementation
│       ├── hyperv_powershell.go    # PowerShell execution
│       ├── mock.go                 # Mock implementation
│       ├── vmmanager_test.go
│       └── scripts/
│           └── configure-runner.ps1 # Embedded VM startup script
├── go.mod                           # Go dependencies
├── go.sum                           # Dependency checksums
├── config.example.yaml              # Configuration template
├── start.ps1                        # Windows startup script (optional)
├── .goreleaser.yml                  # GoReleaser configuration
├── Taskfile.yml                     # Task automation
├── main.go                          # Legacy (deprecated)
├── main_test.go                     # Legacy (deprecated)
├── packer/
│   ├── runner-customizations/
│   │   ├── runner-basic.pkr.hcl    # Minimal Packer template
│   │   └── runner-enhanced.pkr.hcl # Full-featured Packer template
│   ├── autounattend.iso            # Windows unattended install
│   └── scripts/
│       └── (various setup scripts)
└── .github/
    └── workflows/
        ├── test.yml                # Test workflow
        └── release.yml             # Release automation
```

### Package Organization

Following Go best practices, the code is organized into modular packages:

- **[cmd/](cmd/)**: Command-line applications
- **[pkg/](pkg/)**: Reusable library packages
  - **config**: Configuration loading and validation
  - **github**: GitHub API interactions
  - **logger**: Structured logging setup
  - **orchestrator**: VM pool lifecycle management
  - **vmmanager**: VM operations (Hyper-V and mock implementations)

See [pkg/README.md](pkg/README.md) and [cmd/README.md](cmd/README.md) for detailed documentation.

## Security Considerations

- **GitHub App credentials** should be kept secure (private key file)
- **Short-lived tokens**: App generates fresh tokens for each VM (expire in 1 hour)
- **VMs are ephemeral**: No persistent state between jobs
- **Template VHDX** should be read-only to prevent tampering
- **No inbound network access** required - orchestrator polls locally
- **Config injection**: Sensitive tokens written to VHDX temporarily (unmounted after)
- **Runs entirely on local network**: No external webhook endpoints to secure
- **YAML config**: Keep `config.yaml` secure (contains private key path and credentials)

## Performance Metrics

### Disk Creation Performance

This project uses **Hyper-V differencing disks** (child/parent VHDX architecture) for optimal performance:

- **VM Creation Time**: ~1-2 seconds (differencing disk)
  - Previous (full copy): ~15 seconds per VM
  - **90% faster** with differencing disks
- **VM Destruction Time**: ~5 seconds
- **Pool Initialization**: ~5-10 seconds (4 VMs in parallel)
  - Previous (full copy): ~60 seconds
  - **85% faster** pool startup
- **Job Pickup Time**: < 5 seconds (warm pool ready)

### Storage Efficiency

- **Per-VM Storage**: 2-5 GB (child disk stores only changes)
  - Previous (full copy): 30-75 GB per VM
  - **85-95% storage reduction**
- **4-VM Pool**: ~15-25 GB total (template + 4 children)
  - Previous (full copy): 120-300 GB total

### How Differencing Disks Work

Each VM gets a small "child" VHDX that references the read-only "parent" template:
- **Parent Template**: Read-only base image (30-75 GB) shared by all VMs
- **Child Disks**: Write-only per-VM disks (2-5 GB) storing only changes
- **Copy-on-Write**: First write to each block triggers copy from parent
- **Near-Instant Creation**: `New-VHD -Differencing` creates metadata only (~1s)
- **Automatic**: Packer build automatically sets template to read-only

## License

MIT License - See LICENSE file for details

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `go test -v ./...`
5. Submit a pull request

## Support

For issues and questions:
- Open an issue on GitHub
- Check troubleshooting section above

---

**Built with ❤️ for efficient GitHub Actions self-hosted runners**

## Legal
   
   This project is not affiliated with, endorsed by, or sponsored by Microsoft Corporation.
   Hyper-V is a trademark of Microsoft Corporation.
