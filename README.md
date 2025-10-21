# Hyper-V Runner Pool

A production-ready pool manager for running GitHub Actions workflows in ephemeral Hyper-V VMs on Windows. Each job runs in a fresh, isolated VM that is automatically destroyed and recreated after completion.

## Features

- **Ephemeral VMs**: Fresh VM for every job - zero state leakage between runs
- **Concurrent Execution**: Pool of VMs ready to handle multiple jobs simultaneously
- **Automatic Lifecycle Management**: VMs are created, registered, and destroyed automatically
- **Serverless Polling**: Monitors VM state locally - no external network required
- **Cross-Platform Development**: Develop and test on macOS, deploy to Windows
- **Production Ready**: Robust error handling, structured logging, and concurrent operations
- **Flexible Images**: Choose between minimal (fast) or enhanced (GitHub-compatible) VM templates
- **Air-Gappable**: Works on isolated networks with no inbound internet access

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
cd warc

# Dependencies are already installed via go.mod
go mod download
```

### 2. Create Environment Configuration

```bash
cp .env.example .env
# Edit .env and set:
#   - GITHUB_PAT (your Personal Access Token)
#   - GITHUB_ORG (your organization)
#   - GITHUB_REPO (your repository, or leave empty for org runners)
#   - USE_MOCK=true (for macOS testing)
```

### 3. Run with Mock VM Manager

```bash
# Option 1: Using Task (recommended)
task run

# Option 2: Manual
export USE_MOCK=true
go run main.go
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
GOOS=windows GOARCH=amd64 go build -o hyperv-runner-pool.exe
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
- `.env.example` - Configuration template
- `start.ps1` - Startup script
- `README.md` - Documentation
- `packer/` - Complete Packer configuration
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
cd warc

# Run bootstrap (as Administrator)
.\bootstrap.ps1
```

The bootstrap script will:
- Install Chocolatey, Go, Packer, Task, GoReleaser, and Git
- Enable Hyper-V (with reboot prompt)
- Create `vms\templates\` and `vms\storage\` directories
- Set up your `.env` file
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

   To use a custom location, set these in your `.env` file:
   ```
   VM_TEMPLATE_PATH=C:\custom\path\runner-template.vhdx
   VM_STORAGE_PATH=C:\custom\storage\path
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

### Phase 3: Configure GitHub

**Generate Personal Access Token**
- Go to GitHub Settings → Developer settings → Personal access tokens
- For repository runners: `repo` scope
- For organization runners: `admin:org` → `manage_runners:org`

### Phase 4: Run Orchestrator

```powershell
# Copy binary from macOS build or build on Windows
# go build -o hyperv-runner-pool.exe

# Create .env file with real values
Copy-Item .env.example .env
notepad .env  # Fill in real values

# Run
.\start.ps1
```

## Configuration Reference

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GITHUB_PAT` | Yes | GitHub Personal Access Token |
| `GITHUB_ORG` | Yes | GitHub organization name |
| `GITHUB_REPO` | No | Repository name (empty for org runners) |
| `USE_MOCK` | No | Use mock VMs for testing (true/false) |
| `LOG_LEVEL` | No | Logging level: debug, info, warn, error (default: info) |
| `LOG_FORMAT` | No | Log format: text, json (default: text) |
| `VM_TEMPLATE_PATH` | No | Full path to template VHDX (default: `.\vms\templates\runner-template.vhdx`) |
| `VM_STORAGE_PATH` | No | Directory for VM storage (default: `.\vms\storage`) |

### VM Pool Configuration

The application uses sensible defaults:
- **Pool Size**: 4 concurrent VMs (edit `main.go` to adjust)
- **Template Path**: `.\vms\templates\runner-template.vhdx` (override with `VM_TEMPLATE_PATH`)
- **Storage Path**: `.\vms\storage` (override with `VM_STORAGE_PATH`)

**Why Local Paths?**
- Self-contained: Everything stays in the repository
- Easy cleanup: Delete the repo, delete everything
- Gitignored: VM files won't be committed
- Override-friendly: Set custom paths via environment variables

## How It Works

1. **Orchestrator starts** and creates a warm pool of VMs
2. **For each VM**:
   - Generates fresh GitHub runner token via API
   - Mounts VHDX, injects `runner-config.json`, unmounts
   - Creates and starts VM
   - VM boots and reads config from injected file
   - VM registers with GitHub as ephemeral runner
3. **GitHub assigns jobs** to available runners
4. **Runner executes job** with `--once` flag (single job mode)
5. **Job completes**, runner exits, VM shuts down
6. **Orchestrator detects shutdown** via polling (every 10s)
7. **VM is destroyed and recreated** with fresh token
8. **Cycle repeats** indefinitely

No webhooks, no inbound network access required.

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
vim main.go

# 2. Run tests
go test -v ./...

# 3. Test with mock VMs
USE_MOCK=true go run main.go

# 4. Build for Windows
GOOS=windows GOARCH=amd64 go build -o hyperv-runner-pool.exe

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
2. Use differencing disks (advanced)
3. Reduce pool size if system is overloaded
4. Monitor disk I/O
5. Adjust polling interval if recreation is too slow

## Project Structure

```
.
├── main.go                          # Main application
├── main_test.go                     # Unit tests
├── go.mod                           # Go dependencies
├── go.sum                           # Dependency checksums
├── .env.example                     # Environment template
├── start.ps1                        # Windows startup script
├── .goreleaser.yml                  # GoReleaser configuration
├── Taskfile.yml                     # Task automation
├── packer/
│   ├── windows-runner.pkr.hcl      # Packer template
│   ├── autounattend.xml            # Windows unattended install
│   └── scripts/
│       ├── setup.ps1               # WinRM setup
│       ├── install-runner.ps1      # Runner installation
│       └── configure-startup.ps1   # Startup configuration
└── .github/
    └── workflows/
        ├── test.yml                # Test workflow
        └── release.yml             # Release automation
```

## Security Considerations

- GitHub PAT should be kept secure and rotated regularly
- VMs are ephemeral - no persistent state between jobs
- Template VHDX should be read-only to prevent tampering
- No inbound network access required - orchestrator polls locally
- Config injection writes sensitive tokens to VHDX temporarily (unmounted after)
- Runs entirely on local network - no external webhook endpoints to secure

## Performance Metrics

- **VM Creation Time**: ~15 seconds (with template copy)
- **VM Destruction Time**: ~5 seconds
- **Pool Initialization**: ~60 seconds (4 VMs)
- **Job Pickup Time**: < 5 seconds (warm pool)

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
