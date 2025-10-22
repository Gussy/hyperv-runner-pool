# Command Line Applications

This directory contains the main CLI applications for the project, following the standard Go project layout.

## `hyperv-runner-pool/`

The main application for managing a pool of ephemeral Hyper-V VMs for GitHub Actions runners.

### Building

Build the application:
```bash
go build ./cmd/hyperv-runner-pool
```

Or build and install to `$GOPATH/bin`:
```bash
go install ./cmd/hyperv-runner-pool
```

### Running

Run with a configuration file:
```bash
./hyperv-runner-pool --config config.yaml
```

Or use the short flag:
```bash
./hyperv-runner-pool -c config.yaml
```

### Flags

- `--config, -c`: Path to YAML configuration file (required)

### Example

```bash
# Build the application
go build ./cmd/hyperv-runner-pool

# Run with your config
./hyperv-runner-pool.exe --config config.yaml
```

## Version Information

Version information is automatically set during the build process by GoReleaser:
- `version`: The release version
- `commit`: The git commit hash
- `date`: The build date

Check the version:
```bash
./hyperv-runner-pool --version
```
