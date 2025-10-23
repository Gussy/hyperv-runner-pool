package vmmanager

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// RunPowerShell executes a PowerShell command by writing it to a temp file and executing it
// This approach is more robust than -Command for multi-line scripts and avoids escaping issues
func (h *HyperVManager) RunPowerShell(command string) (string, error) {
	// Create a temporary PowerShell script file
	tempFile, err := os.CreateTemp("", "hyperv-runner-*.ps1")
	if err != nil {
		return "", fmt.Errorf("failed to create temp script file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write the command to the temp file
	if _, err := tempFile.WriteString(command); err != nil {
		tempFile.Close()
		return "", fmt.Errorf("failed to write to temp script file: %w", err)
	}
	tempFile.Close()

	// Log the command at debug level (truncate if very long)
	commandPreview := command
	if len(commandPreview) > 200 {
		commandPreview = commandPreview[:200] + "... (truncated)"
	}
	h.logger.Debug("Executing PowerShell script",
		"script_file", tempFile.Name(),
		"command_preview", commandPreview,
		"command_length", len(command))

	// Optionally save to debug directory for manual testing
	if debugDir := os.Getenv("POWERSHELL_DEBUG_DIR"); debugDir != "" {
		timestamp := time.Now().Format("20060102-150405.000")
		debugFile := fmt.Sprintf("%s\\ps-%s.ps1", debugDir, timestamp)
		if err := os.WriteFile(debugFile, []byte(command), 0644); err != nil {
			h.logger.Warn("Failed to save debug script", "path", debugFile, "error", err)
		} else {
			h.logger.Debug("Saved PowerShell script to debug directory", "path", debugFile)
		}
	}

	// Execute PowerShell with -File parameter (more robust than -Command)
	// Use -WindowStyle Hidden to prevent PowerShell windows from appearing
	cmd := exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-NoProfile", "-WindowStyle", "Hidden", "-File", tempFile.Name())

	// On Windows, hide the console window for the PowerShell process
	// This prevents PowerShell windows from flashing on screen
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	// Capture stdout and stderr separately for better debugging
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	if err != nil {
		// Build detailed error message
		errMsg := fmt.Sprintf("powershell error: %v", err)
		if len(stdoutStr) > 0 {
			errMsg += fmt.Sprintf("\nstdout: %s", stdoutStr)
		}
		if len(stderrStr) > 0 {
			errMsg += fmt.Sprintf("\nstderr: %s", stderrStr)
		}
		errMsg += fmt.Sprintf("\nscript_file: %s (saved for debugging)", tempFile.Name())
		errMsg += fmt.Sprintf("\ncommand_preview: %s", commandPreview)

		return stdoutStr + stderrStr, fmt.Errorf("%s", errMsg)
	}

	// Return combined output (stdout + stderr)
	output := stdoutStr
	if len(stderrStr) > 0 {
		output += stderrStr
	}

	h.logger.Debug("PowerShell script executed successfully",
		"output_length", len(output))

	return output, nil
}
