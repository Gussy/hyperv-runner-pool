# Minimal Windows Runner Image

> **Config:** `windows-runner.pkr.hcl` | **Build Time:** 30-45 min | **Disk:** 30GB total (~14GB free)

## Disk Usage

- **Total Allocated:** 30GB
- **OS + Tools:** ~14-16GB
- **Available for builds:** ~14-16GB
- **Matches:** GitHub's stated 14GB available space

## Installed Software

- **Base:** Windows Server 2022 Evaluation
- **Git** (latest)
- **Node.js** LTS (latest single version)
- **Python** (latest)
- **.NET SDK** (latest)
- **7zip**
- **curl**
- **wget**
- **Chocolatey** (package manager)
- **GitHub Actions Runner** (latest, ephemeral mode)

## Build

```powershell
packer build windows-runner.pkr.hcl
```

## Best For

- Fast iteration
- Custom workflows
- Cost-sensitive deployments
- Controlled dependencies
