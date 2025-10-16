# Enhanced Windows Runner Image

> **Config:** `windows-runner-enhanced.pkr.hcl` | **Build Time:** 2-4 hours | **Disk:** 75GB total (~14GB free)

## Disk Usage

- **Total Allocated:** 75GB
- **OS + Software:** ~55-61GB
- **Available for builds:** ~14-20GB
- **Matches:** GitHub's stated 14GB available space

## Installed Software

### Base System
- Windows Server 2022 Evaluation
- PowerShell 5.1 + PowerShell Core 7.x

### Build Tools
- Visual Studio 2022 Build Tools (VCTools, MSBuild)
- CMake
- Ninja

### Version Control
- Git
- Git LFS
- GitHub CLI (gh)

### Node.js Ecosystem
- Node.js: 16.20.2, 18.19.0, 20.11.0 (via nvm-windows)
- npm, yarn, pnpm

### Python
- Python: 3.9.13, 3.10.13, 3.11.8
- pip, setuptools, wheel (for each version)

### .NET
- .NET SDK: 6.x, 7.x, 8.x

### Java
- OpenJDK: 11, 17
- Maven
- Gradle

### Other Languages
- Ruby 3.2 + Bundler
- Go 1.21
- Rust (latest)

### Cloud CLIs
- AWS CLI v2
- Azure CLI
- Google Cloud SDK (gcloud)

### Container Tools
- Docker Desktop
- kubectl
- Helm

### Infrastructure as Code
- Terraform
- Packer

### Databases
- MySQL Workbench
- PostgreSQL (client + server)
- MongoDB (client + server)
- Redis (client + server)

### Utilities
- 7zip, curl, wget
- jq, yq
- NuGet CLI
- Visual C++ Redistributables (all versions)

### GitHub Actions
- GitHub Actions Runner (latest, ephemeral mode)

## Build

```powershell
packer build windows-runner-enhanced.pkr.hcl
```

## Best For

- Drop-in GitHub-hosted runner replacement
- Production deployments
- Migrating existing workflows
- Diverse tech stacks
- No workflow modifications needed

## Note

This image closely mirrors GitHub's `windows-2022` hosted runner, providing maximum compatibility with existing workflows.
