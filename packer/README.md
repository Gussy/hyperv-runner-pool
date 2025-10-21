# GitHub Actions Runner - Packer Templates

Production-ready Packer templates for building Windows Server 2022 GitHub Actions runner images on Hyper-V, leveraging the battle-tested [hv-packer](https://github.com/marcinbojko/hv-packer) framework.

## Architecture

This project uses **hv-packer as a git submodule** to provide robust Windows Server provisioning, then extends it with GitHub Actions runner-specific customizations.

### Why This Approach?

- **Battle-tested foundation**: hv-packer provides proven Windows Server 2022 builds with proper phase management
- **Maintained upstream**: Get security updates and improvements from hv-packer
- **Runner-specific extensions**: Our customizations add GitHub Actions runner installation on top
- **Separation of concerns**: Base OS provisioning vs. runner configuration

### Directory Structure

```
packer/
├── hv-packer/                          # Git submodule - proven Hyper-V templates
│   ├── templates/                      # Base Packer templates
│   ├── extra/scripts/windows/shared/   # Phase 1-5 provisioning scripts
│   └── variables/                      # Windows Server configurations
├── runner-customizations/              # Our GitHub Actions runner extensions
│   ├── provisioners/                   # Runner installation scripts
│   │   ├── install-runner.ps1
│   │   ├── configure-startup.ps1
│   │   └── install-enhanced-tools.ps1
│   ├── variables/                      # Runner build configurations
│   │   ├── runner-basic.pkvars.hcl
│   │   └── runner-enhanced.pkvars.hcl
│   ├── runner-basic.pkr.hcl           # Basic runner template
│   └── runner-enhanced.pkr.hcl        # Enhanced runner template
├── build-runner.ps1                    # Build orchestration script
└── Taskfile.yml                        # Task automation
```

## Build Types

### Basic Runner

**Ideal for**: General-purpose CI/CD workloads

- **Build time**: ~1.5-2 hours
- **Disk size**: 40GB
- **Included tools**:
  - Git
  - Node.js LTS
  - Python (latest)
  - .NET SDK (latest)
  - 7zip, curl, wget

### Enhanced Runner

**Ideal for**: Full-stack development, polyglot projects

- **Build time**: ~3-4 hours
- **Disk size**: 80GB
- **Included tools**:
  - Everything in Basic, plus:
  - Visual Studio 2022 Build Tools
  - Multiple Node.js versions (16, 18, 20) via nvm
  - Multiple Python versions (3.9, 3.10, 3.11)
  - .NET SDK (6, 7, 8)
  - Java (11, 17) + Maven, Gradle
  - Ruby 3.2 + Bundler
  - Go 1.21
  - Rust
  - PowerShell Core
  - Cloud CLIs (AWS, Azure, Google Cloud)
  - Container tools (Docker Desktop, kubectl, Helm)
  - Database clients (MySQL, PostgreSQL, MongoDB, Redis)
  - Build tools (CMake, Ninja, Terraform, Packer)

## Prerequisites

1. **Windows with Hyper-V**: Windows 10/11 Pro or Windows Server with Hyper-V role
2. **Packer**: Version 1.10.0 or higher ([Download](https://www.packer.io/downloads))
3. **Git**: For submodule management
4. **Hyper-V Switch**: External switch or switch with NAT/DHCP configured
5. **Administrator Rights**: Required for Hyper-V operations
6. **Disk Space**: 100GB+ free for build artifacts

## Quick Start

```powershell
# 1. Clone the repository (with submodules)
git clone --recurse-submodules <your-repo-url>
cd hyperv-runner-pool/packer

# 2. Initialize Packer plugins and verify submodule
task init

# 3. View available builds
task info

# 4. Validate configuration (optional but recommended)
task validate:basic

# 5. Build basic runner image
task build:basic

# OR build enhanced runner image
task build:enhanced
```

## Build Process Details

### What hv-packer Provides

The base hv-packer templates handle:

1. **Phase 1**: Initial system configuration (networking, Windows features)
2. **Phase 2**: Additional system setup (performance tuning, services)
3. **Windows Updates**: Two-pass update cycle for comprehensive patching
4. **Phase 5a**: Base software installation (Chocolatey, PowerShell modules)
5. **Phase 5d**: Disk compression and optimization
6. **Sysprep**: Generalization for template reuse

### Our Runner Customizations

On top of hv-packer's foundation, we add:

1. **Development tools**: Language runtimes and build tools
2. **GitHub Actions Runner**: Pre-installed but unconfigured
3. **Startup automation**: Scripts for runner registration on first boot
4. **Runner-specific optimizations**: Paths, permissions, service configuration

## Configuration

### Customizing the Build

Edit variable files in [runner-customizations/variables/](runner-customizations/variables/):

```hcl
# runner-basic.pkvars.hcl or runner-enhanced.pkvars.hcl

# Change Hyper-V switch
switch_name="Your-Switch-Name"

# Adjust VM resources
cpus="8"
memory="16384"  # in MB
disk_size="100000"  # in MB

# Use different Windows Server ISO
iso_url="https://..."
iso_checksum="sha256:..."
```

### Adding Custom Tools

Edit the provisioner scripts in [runner-customizations/provisioners/](runner-customizations/provisioners/):

```powershell
# install-enhanced-tools.ps1

# Add your custom tool installation
Write-Host '=== Installing Custom Tool ===' -ForegroundColor Cyan
choco install -y your-tool-here
Write-Host 'Custom tool installed' -ForegroundColor Green
```

## Available Tasks

Run `task --list` to see all available tasks:

| Task | Description |
|------|-------------|
| `task init` | Initialize submodule and Packer plugins |
| `task info` | Display build information |
| `task validate:basic` | Validate basic runner config |
| `task validate:enhanced` | Validate enhanced runner config |
| `task build:basic` | Build basic runner |
| `task build:enhanced` | Build enhanced runner |
| `task build:basic:debug` | Build basic with debug logging |
| `task build:enhanced:debug` | Build enhanced with debug logging |
| `task clean` | Clean build artifacts |

## Troubleshooting

### Submodule Not Initialized

```
ERROR: hv-packer submodule not initialized
```

**Solution**:
```powershell
git submodule update --init --recursive
```

### Build Fails During Phase 1-5

Check hv-packer's logs and scripts:
- Review [hv-packer/extra/scripts/windows/shared/](hv-packer/extra/scripts/windows/shared/)
- Enable debug logging: `task build:basic:debug`
- Check Windows Event Viewer on the building VM

### WinRM Connection Timeout

hv-packer handles WinRM configuration in its autounattend.xml and bootstrap scripts. If issues persist:

1. Verify the Hyper-V switch has network connectivity
2. Check Windows Firewall isn't blocking WinRM (ports 5985/5986)
3. Review [hv-packer/extra/files/windows/2022/hyperv/std/Autounattend.xml](hv-packer/extra/files/windows/2022/hyperv/std/Autounattend.xml)

### Image Edition Mismatch

```
ERROR: Windows could not apply the settings in the unattend file
```

The hv-packer secondary ISO's Autounattend.xml specifies the Windows edition. To verify:

```powershell
# Mount your Windows Server ISO
$iso = Mount-DiskImage -ImagePath "path\to\your.iso" -PassThru
$drive = ($iso | Get-Volume).DriveLetter

# List available editions
Get-WindowsImage -ImagePath "${drive}:\sources\install.wim"

# Dismount
Dismount-DiskImage -ImagePath "path\to\your.iso"
```

If you need a different edition, you'll need to rebuild hv-packer's secondary ISO.

### Packer Plugin Issues

```
ERROR: Failed to initialize plugins
```

**Solution**:
```powershell
# Re-initialize Packer plugins
packer init runner-customizations/runner-basic.pkr.hcl
packer init runner-customizations/runner-enhanced.pkr.hcl
```

## Advanced Usage

### Direct Build Script Usage

```powershell
# Basic build with logging
.\build-runner.ps1 -BuildType basic -EnableLogging

# Enhanced build, validate only
.\build-runner.ps1 -BuildType enhanced -ValidateOnly
```

### Manual Packer Commands

```powershell
cd packer

# Validate
packer validate `
  -var-file="runner-customizations/variables/runner-basic.pkvars.hcl" `
  runner-customizations/runner-basic.pkr.hcl

# Build
packer build --force `
  -var-file="runner-customizations/variables/runner-basic.pkvars.hcl" `
  runner-customizations/runner-basic.pkr.hcl
```

## Updating hv-packer

To get the latest updates from hv-packer:

```powershell
cd packer/hv-packer
git pull origin master
cd ../..
git add packer/hv-packer
git commit -m "Update hv-packer submodule"
```

## Migration from Old Setup

If you were using the previous packer setup:

1. **Old files are preserved**: Your old [windows-runner.pkr.hcl](windows-runner.pkr.hcl) and scripts are still in the packer directory
2. **New structure**: The new hv-packer integration is in [runner-customizations/](runner-customizations/)
3. **Task changes**: The Taskfile.yml now uses the new build process
4. **No autounattend ISO needed**: hv-packer provides the secondary ISO

## References

- **hv-packer**: https://github.com/marcinbojko/hv-packer
- **Packer Hyper-V ISO Builder**: https://www.packer.io/plugins/builders/hyperv/iso
- **Windows Unattended Installation**: https://learn.microsoft.com/en-us/windows-hardware/customize/desktop/unattend/

## Contributing

When contributing:

1. **Don't modify hv-packer submodule files** - Submit PRs to the upstream repo
2. **Runner customizations only**: Focus changes on [runner-customizations/](runner-customizations/)
3. **Test both builds**: Validate changes work for basic and enhanced builds
4. **Document**: Update this README for significant changes

## Support

For issues:

1. **hv-packer base problems**: Report at https://github.com/marcinbojko/hv-packer/issues
2. **Runner-specific issues**: Report in this repository's issues
3. **Logs**: Enable debug logging with `task build:basic:debug` or `task build:enhanced:debug`
