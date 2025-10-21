# Uploading Packer Images to GitHub Container Registry

This guide explains how to upload your Packer-built VHDX images to GitHub Container Registry (ghcr.io) for storage and versioning using ORAS (OCI Registry as Storage).

## Why Use ghcr.io for VHDX Storage?

- **Free storage** for public repositories
- **Version control** with tags (latest, dated versions, semantic versions)
- **Built-in authentication** via GitHub tokens
- **Standard OCI tooling** - works with any OCI-compatible registry
- **Package permissions** linked to your GitHub repository

## Prerequisites

### 1. Install ORAS (OCI Registry as Storage)

Using Chocolatey:
```powershell
choco install oras
```

Or download manually from [ORAS releases](https://github.com/oras-project/oras/releases).

Verify installation:
```powershell
oras version
```

### 2. Create a GitHub Personal Access Token

1. Go to https://github.com/settings/tokens
2. Click "Generate new token" → "Generate new token (classic)"
3. Select the following scopes:
   - `write:packages` - Upload packages
   - `read:packages` - Download packages
   - `delete:packages` (optional) - Delete package versions
4. Click "Generate token" and copy it

### 3. Authenticate to ghcr.io

```powershell
# Store your token in an environment variable (for this session)
$env:GITHUB_TOKEN = "ghp_your_token_here"

# Login to GitHub Container Registry
$env:GITHUB_TOKEN | oras login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

To persist the token across sessions, add it to your environment variables or use a credential manager.

## Pushing Images

After a successful Packer build, the VHDX files will be in the output directory.

### Basic Runner Example

```powershell
# Navigate to the output directory
cd output/runner-basic/Virtual Hard Disks

# Push with 'latest' tag
oras push ghcr.io/YOUR_USERNAME/runner-windows-basic:latest `
  runner.vhdx:application/vnd.hyperv.vhdx `
  --annotation "org.opencontainers.image.created=$(Get-Date -Format o)" `
  --annotation "org.opencontainers.image.description=GitHub Actions Runner - Basic"

# Or push with a date-based version tag
$VERSION = Get-Date -Format "yyyyMMdd-HHmmss"
oras push ghcr.io/YOUR_USERNAME/runner-windows-basic:$VERSION `
  runner.vhdx:application/vnd.hyperv.vhdx `
  --annotation "org.opencontainers.image.created=$(Get-Date -Format o)" `
  --annotation "org.opencontainers.image.description=GitHub Actions Runner - Basic"
```

### Enhanced Runner Example

```powershell
cd output/runner-enhanced/Virtual Hard Disks

oras push ghcr.io/YOUR_USERNAME/runner-windows-enhanced:latest `
  runner.vhdx:application/vnd.hyperv.vhdx `
  --annotation "org.opencontainers.image.created=$(Get-Date -Format o)" `
  --annotation "org.opencontainers.image.description=GitHub Actions Runner - Enhanced"
```

### Using Organization Repositories

If you're part of a GitHub organization:

```powershell
oras push ghcr.io/YOUR_ORG/runner-windows-basic:latest `
  runner.vhdx:application/vnd.hyperv.vhdx
```

## Pulling Images

When you need to use the image on another machine:

```powershell
# Create a directory for the image
mkdir runner-image
cd runner-image

# Pull the latest version
oras pull ghcr.io/YOUR_USERNAME/runner-windows-basic:latest

# Pull a specific version
oras pull ghcr.io/YOUR_USERNAME/runner-windows-basic:20250121-143022

# The VHDX file will be downloaded to the current directory
```

## Managing Images

### List Available Versions

```powershell
# View all tags for an image
oras repo tags ghcr.io/YOUR_USERNAME/runner-windows-basic

# View image manifest details
oras manifest fetch ghcr.io/YOUR_USERNAME/runner-windows-basic:latest
```

### Delete a Version

```powershell
# Delete a specific tag (requires delete:packages permission)
oras delete ghcr.io/YOUR_USERNAME/runner-windows-basic:20250121-143022
```

### Make Package Public or Private

1. Go to your GitHub profile → Packages
2. Select the package (e.g., `runner-windows-basic`)
3. Click "Package settings"
4. Under "Danger Zone", choose "Change visibility"

## Automation Example

Here's a PowerShell script to automate the upload process:

```powershell
# upload-to-ghcr.ps1
param(
    [Parameter(Mandatory=$true)]
    [string]$ImageType,  # "basic" or "enhanced"

    [Parameter(Mandatory=$true)]
    [string]$Username,

    [string]$Token = $env:GITHUB_TOKEN
)

# Authenticate if not already logged in
if ($Token) {
    $Token | oras login ghcr.io -u $Username --password-stdin
}

# Set paths
$outputDir = "output/runner-$ImageType/Virtual Hard Disks"
$imageName = "ghcr.io/$Username/runner-windows-$ImageType"

# Generate version tag
$dateVersion = Get-Date -Format "yyyyMMdd-HHmmss"
$created = Get-Date -Format o

# Push with both 'latest' and date version tags
Push-Location $outputDir
try {
    Write-Host "Pushing $imageName:latest..."
    oras push "${imageName}:latest" `
        runner.vhdx:application/vnd.hyperv.vhdx `
        --annotation "org.opencontainers.image.created=$created" `
        --annotation "org.opencontainers.image.description=GitHub Actions Runner - $ImageType"

    Write-Host "Pushing $imageName:$dateVersion..."
    oras push "${imageName}:$dateVersion" `
        runner.vhdx:application/vnd.hyperv.vhdx `
        --annotation "org.opencontainers.image.created=$created" `
        --annotation "org.opencontainers.image.description=GitHub Actions Runner - $ImageType"

    Write-Host "Successfully uploaded image with tags: latest, $dateVersion"
} finally {
    Pop-Location
}
```

Usage:
```powershell
.\upload-to-ghcr.ps1 -ImageType basic -Username your-username
```

## Important Notes

- **File Sizes**: VHDX files are large (40-80GB for these runners). Upload times will vary based on your internet connection.
- **Rate Limits**: ghcr.io has rate limits for unauthenticated pulls. Authentication is recommended for downloading.
- **Storage Format**: Images are stored as OCI artifacts, not container images. They won't appear in Docker/Podman.
- **Permissions**: Packages inherit permissions from the repository. Configure package permissions separately in GitHub settings.
- **Bandwidth**: Consider your bandwidth usage, especially if uploading frequently or on metered connections.

## Troubleshooting

### Authentication Failed
```powershell
# Check if you're logged in
oras login ghcr.io --username YOUR_USERNAME

# Verify your token has the correct permissions
# Token needs: write:packages, read:packages
```

### File Not Found
```powershell
# Verify the VHDX file exists
Get-ChildItem "output/runner-*/Virtual Hard Disks/*.vhdx"

# Check you're in the correct directory
Get-Location
```

### Push Failed - File Too Large
- ghcr.io supports large files, but network timeouts can occur
- Try using a wired connection instead of WiFi
- Consider splitting the upload process or using a more stable connection

## Additional Resources

- [ORAS Documentation](https://oras.land/)
- [GitHub Container Registry Documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [OCI Artifacts Specification](https://github.com/opencontainers/artifacts)
