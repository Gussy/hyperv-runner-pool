package tray

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"fyne.io/systray"
)

// Controller defines the interface for orchestrator operations
type Controller interface {
	RestartAllVMs() error
	Shutdown() error
}

// Config holds the system tray configuration
type Config struct {
	Controller Controller
	Logger     *slog.Logger
	OnReady    func() // Called when tray is ready
}

var globalConfig Config

// Run starts the system tray application (blocking call)
// This must be called from the main goroutine
func Run(cfg Config) {
	globalConfig = cfg
	systray.Run(onReady, onExit)
}

// onReady is called when the system tray is ready
func onReady() {
	cfg := globalConfig

	// Set the icon
	systray.SetIcon(Icon)
	systray.SetTitle("Hyper-V Runner Pool")
	systray.SetTooltip("Hyper-V Runner Pool Manager")

	cfg.Logger.Info("System tray initialized")

	// Add menu items
	mRestartVMs := systray.AddMenuItem("Restart All VMs", "Restart all VMs in the pool")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Exit", "Exit the application")

	// Signal that tray is ready
	if cfg.OnReady != nil {
		go cfg.OnReady()
	}

	// Setup signal handling in the background
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		cfg.Logger.Info("Received shutdown signal, exiting")
		systray.Quit()
	}()

	// Handle menu item clicks in a goroutine
	go func() {
		for {
			select {
			case <-mRestartVMs.ClickedCh:
				cfg.Logger.Info("Restart All VMs requested from system tray")
				if err := cfg.Controller.RestartAllVMs(); err != nil {
					cfg.Logger.Error("Failed to restart VMs", "error", err)
				} else {
					cfg.Logger.Info("All VMs restart initiated successfully")
				}

			case <-mQuit.ClickedCh:
				cfg.Logger.Info("Exit requested from system tray")
				systray.Quit()
			}
		}
	}()
}

// onExit is called when the system tray is exiting
func onExit() {
	cfg := globalConfig
	cfg.Logger.Info("System tray exiting, performing shutdown")

	// Perform cleanup
	if cfg.Controller != nil {
		if err := cfg.Controller.Shutdown(); err != nil {
			cfg.Logger.Error("Error during shutdown", "error", err)
		}
	}
}
