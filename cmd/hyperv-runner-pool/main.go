package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"

	"hyperv-runner-pool/pkg/config"
	"hyperv-runner-pool/pkg/github"
	"hyperv-runner-pool/pkg/logger"
	"hyperv-runner-pool/pkg/orchestrator"
	"hyperv-runner-pool/pkg/tray"
	"hyperv-runner-pool/pkg/vmmanager"
)

// Version information (set by GoReleaser during build)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	app := &cli.Command{
		Name:    "hyperv-runner-pool",
		Usage:   "Manage a pool of ephemeral Hyper-V VMs for GitHub Actions runners",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Usage:    "Path to YAML configuration file",
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "no-tray",
				Usage:   "Disable system tray icon (console mode)",
				Sources: cli.EnvVars("NO_TRAY"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			configPath := cmd.String("config")

			// Load configuration from YAML file
			cfg, err := config.LoadFromFile(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Setup logger with config
			log := logger.Setup(cfg.Debug.LogLevel, cfg.Debug.LogFormat, cfg.Debug.LogDirectory)

			// Print version information
			log.Info("Starting Hyper-V Runner Pool",
				"version", version,
				"commit", commit,
				"built", date)

			log.Info("Configuration loaded",
				"config_file", configPath,
				"pool_size", cfg.Runners.PoolSize,
				"mock_mode", cfg.Debug.UseMock)
			log.Info("Using template path", "path", cfg.HyperV.TemplatePath)
			log.Info("Using storage path", "path", cfg.HyperV.VMStoragePath)

			// Determine VM manager based on config
			var vmMgr vmmanager.VMManager

			if cfg.Debug.UseMock {
				log.Info("Using Mock VM Manager (development mode)")
				vmMgr = vmmanager.NewMockVMManager(log)
			} else {
				log.Info("Using Hyper-V VM Manager (production mode)")
				vmMgr = vmmanager.NewHyperVManager(*cfg, log)
			}

			// Create GitHub client
			ghClient := github.NewClient(*cfg, log)

			// Create orchestrator
			orch := orchestrator.New(*cfg, vmMgr, ghClient, log)

			// Check if system tray is disabled
			noTray := cmd.Bool("no-tray")

			if noTray {
				// Console mode: Initialize pool and wait for signal
				log.Info("Orchestrator running in console mode (no system tray)")

				// Initialize VM pool
				if err := orch.InitializePool(); err != nil {
					log.Error("Failed to initialize pool", "error", err)
					log.Warn("Some VMs may not be ready, but continuing to run. Press Ctrl+C to shutdown.")
				} else {
					log.Info("Pool initialized successfully")
				}

				log.Info("Press Ctrl+C to shutdown gracefully")

				// Setup signal handling for graceful shutdown
				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

				// Wait for shutdown signal
				sig := <-sigChan
				log.Info("Received shutdown signal", "signal", sig.String())

				// Perform graceful shutdown
				if err := orch.Shutdown(); err != nil {
					log.Error("Error during shutdown", "error", err)
					return err
				}

				log.Info("Shutdown complete")
			} else {
				// System tray mode: Initialize in background and run tray (blocking)
				log.Info("Starting with system tray icon")
				log.Info("Initializing VM pool in background...")

				// Initialize pool in background before starting tray
				initErr := make(chan error, 1)
				go func() {
					initErr <- orch.InitializePool()
				}()

				// Start system tray (blocking - must be on main goroutine)
				// The tray will handle shutdown when user clicks Exit
				tray.Run(tray.Config{
					Controller: orch,
					Logger:     log,
					OnReady: func() {
						// Check if pool initialization completed successfully
						if err := <-initErr; err != nil {
							log.Error("Failed to initialize pool", "error", err)
							log.Info("Exiting due to initialization error")
							return
						}
						log.Info("VM pool initialized successfully")
						log.Info("System tray active - right-click icon to manage VMs or exit")
					},
				})

				// tray.Run() blocks until systray.Quit() is called
				// Shutdown is handled in the tray's onExit callback
				log.Info("Shutdown complete")
			}

			return nil
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
