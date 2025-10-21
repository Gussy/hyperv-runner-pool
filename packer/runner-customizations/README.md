# GitHub Actions Runner Images

This directory contains Packer templates for building GitHub Actions runner images on Hyper-V, extending [hv-packer](https://github.com/marcinbojko/hv-packer)'s Windows Server 2022 base.

## Available Images

### Basic Runner

**Purpose**: General-purpose CI/CD workloads

**Specifications**:
- **Build time**: ~1.5-2 hours
- **Disk size**: 40GB
- **Memory**: 6GB
- **CPUs**: 4

**Included Tools**:
- Git
- Node.js LTS
- Python (latest)
- .NET SDK (latest)
- 7zip, curl, wget

**Use case**: Standard builds, tests, deployments for Node.js, Python, or .NET projects

---

### Enhanced Runner

**Purpose**: Full-stack development and polyglot projects

**Specifications**:
- **Build time**: ~3-4 hours
- **Disk size**: 80GB
- **Memory**: 8GB
- **CPUs**: 4

**Included Tools**:
- All Basic tools, plus:
- **Build Tools**: Visual Studio 2022 Build Tools (C++, MSBuild)
- **Node.js**: Multiple versions (16, 18, 20) via nvm
- **Python**: Multiple versions (3.9, 3.10, 3.11)
- **.NET**: SDK 6, 7, 8
- **Java**: OpenJDK 11, 17 + Maven, Gradle
- **Languages**: Ruby 3.2, Go 1.21, Rust
- **Cloud**: AWS CLI, Azure CLI, Google Cloud SDK
- **Containers**: Docker Desktop, kubectl, Helm
- **Databases**: MySQL Workbench, PostgreSQL, MongoDB, Redis clients
- **Build Tools**: CMake, Ninja, Terraform, Packer

**Use case**: Complex builds requiring multiple language runtimes, containerization, or cloud deployments

## Structure

```
runner-customizations/
├── config.pkr.hcl              # Packer plugin configuration
├── variables.pkr.hcl           # Shared variable definitions
├── runner-basic.pkr.hcl        # Basic runner template
├── runner-enhanced.pkr.hcl     # Enhanced runner template
├── variables/
│   ├── runner-basic.pkvars.hcl    # Basic build configuration
│   └── runner-enhanced.pkvars.hcl # Enhanced build configuration
└── provisioners/
    ├── install-runner.ps1          # GitHub Actions runner installation
    ├── configure-startup.ps1       # Runner startup configuration
    └── install-enhanced-tools.ps1  # Enhanced toolchain installation
```

## Build Process

Both images follow the same provisioning flow:

1. **hv-packer Phase 1**: Initial system configuration
2. **hv-packer Phase 2**: System setup and tuning
3. **Windows Updates**: Two-pass update cycle
4. **hv-packer Phase 5a**: Base software (Chocolatey, PowerShell modules)
5. **Custom Tools**: Install development tools (basic or enhanced)
6. **GitHub Runner**: Install and configure GitHub Actions runner
7. **hv-packer Phase 5d**: Disk compression and optimization
8. **Sysprep**: Generalize for template reuse

## Customization

### Adding Tools

Edit the provisioner scripts:
- **Basic**: Add inline commands to [runner-basic.pkr.hcl](runner-basic.pkr.hcl#L87-L102)
- **Enhanced**: Edit [provisioners/install-enhanced-tools.ps1](provisioners/install-enhanced-tools.ps1)

### Adjusting Resources

Edit the variable files in [variables/](variables/):
```hcl
cpus = "8"           # Increase CPU cores
memory = "16384"     # Increase RAM (in MB)
disk_size = "100000" # Increase disk (in MB)
```

### Using Different Windows ISO

Update ISO URL and checksum in the variable files:
```hcl
iso_url = "https://..."
iso_checksum = "sha256:..."
```

## Quick Reference

| Task | Command |
|------|---------|
| Build basic runner | `task build:basic` |
| Build enhanced runner | `task build:enhanced` |
| Validate configuration | `task validate:basic` or `task validate:enhanced` |
| Build with debug logs | `task build:basic:debug` |

See [../README.md](../README.md) for complete documentation.
