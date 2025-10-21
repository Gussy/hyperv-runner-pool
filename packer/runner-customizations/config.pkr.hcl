# Packer configuration for GitHub Actions Runner builds
# Declares required plugins

packer {
  required_version = ">= 1.10.0"

  required_plugins {
    # Hyper-V builder
    hyperv = {
      version = ">= 1.1.0"
      source  = "github.com/hashicorp/hyperv"
    }

    # Windows Update provisioner
    windows-update = {
      version = ">= 0.14.1"
      source  = "github.com/rgl/windows-update"
    }
  }
}
