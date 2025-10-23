package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v69/github"

	"hyperv-runner-pool/pkg/config"
)

// Client wraps GitHub API interactions
type Client struct {
	config config.Config
	logger *slog.Logger
}

// NewClient creates a new GitHub API client
func NewClient(cfg config.Config, logger *slog.Logger) *Client {
	return &Client{
		config: cfg,
		logger: logger.With("component", "github"),
	}
}

// getAuthenticatedClient returns an authenticated GitHub client and installation info
func (c *Client) getAuthenticatedClient(ctx context.Context) (*github.Client, *github.Installation, error) {
	// Create GitHub App transport with JWT authentication
	appTransport, err := ghinstallation.NewAppsTransportKeyFromFile(
		http.DefaultTransport,
		c.config.GitHub.AppID,
		c.config.GitHub.AppPrivateKeyPath,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	// Create a GitHub client with app authentication to find installation
	appClient := github.NewClient(&http.Client{Transport: appTransport})

	// List all installations for this GitHub App
	// This approach works for both personal accounts and organizations
	installations, _, err := appClient.Apps.ListInstallations(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list installations: %w", err)
	}

	// Find the installation matching our configured org/user account
	account := c.config.GitHub.GetAccount()
	var installation *github.Installation
	for _, inst := range installations {
		if inst.Account != nil && inst.Account.GetLogin() == account {
			installation = inst
			break
		}
	}

	if installation == nil {
		return nil, nil, fmt.Errorf("GitHub App is not installed on account '%s'. Please install the app at: https://github.com/apps/YOUR_APP_NAME/installations/new", account)
	}

	c.logger.Debug("Found GitHub App installation",
		"installation_id", installation.GetID(),
		"account", account,
		"account_type", installation.Account.GetType())

	// Create installation transport for API calls as this installation
	installationTransport := ghinstallation.NewFromAppsTransport(appTransport, installation.GetID())

	// Create GitHub client authenticated as the installation
	client := github.NewClient(&http.Client{Transport: installationTransport})

	return client, installation, nil
}

// GetRunnerToken generates a GitHub runner registration token using GitHub App authentication
func (c *Client) GetRunnerToken() (string, error) {
	// In mock mode, return a fake token without calling GitHub API
	if c.config.Debug.UseMock {
		mockToken := fmt.Sprintf("mock-runner-token-%d", time.Now().UnixNano())
		c.logger.Debug("Generated mock token", "token", mockToken)
		return mockToken, nil
	}

	ctx := context.Background()

	client, installation, err := c.getAuthenticatedClient(ctx)
	if err != nil {
		return "", err
	}

	// Generate runner registration token
	var token *github.RegistrationToken
	var resp *github.Response

	// Determine if this is a User account (personal) or Organization
	isUserAccount := installation.Account.GetType() == "User"

	if c.config.GitHub.Repo != "" {
		// Repository-level runner (works for both org and user accounts)
		token, resp, err = client.Actions.CreateRegistrationToken(
			ctx,
			c.config.GitHub.GetAccount(),
			c.config.GitHub.Repo,
		)
		if err != nil {
			return "", fmt.Errorf("failed to create repo runner token: %w", err)
		}
	} else if isUserAccount {
		// Personal accounts don't support account-level runners
		return "", fmt.Errorf("personal accounts (User type) require a repository to be specified. Please set 'github.repo' in your config")
	} else {
		// Organization-level runner (only for organizations)
		token, resp, err = client.Actions.CreateOrganizationRegistrationToken(ctx, c.config.GitHub.GetAccount())
		if err != nil {
			return "", fmt.Errorf("failed to create org runner token: %w", err)
		}
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("GitHub API error: unexpected status code %d", resp.StatusCode)
	}

	c.logger.Debug("Generated runner registration token",
		"expires_at", token.GetExpiresAt())

	return token.GetToken(), nil
}

// RunnerInfo contains information about a GitHub Actions runner
type RunnerInfo struct {
	ID     int64
	Name   string
	Status string // "online", "offline"
}

// ListRunners lists all runners for the configured repository or organization
func (c *Client) ListRunners() ([]RunnerInfo, error) {
	// In mock mode, return empty list
	if c.config.Debug.UseMock {
		c.logger.Debug("Mock mode: returning empty runner list")
		return []RunnerInfo{}, nil
	}

	ctx := context.Background()

	client, installation, err := c.getAuthenticatedClient(ctx)
	if err != nil {
		return nil, err
	}

	var runners []RunnerInfo
	isUserAccount := installation.Account.GetType() == "User"

	if c.config.GitHub.Repo != "" {
		// Repository-level runners
		opts := &github.ListRunnersOptions{
			ListOptions: github.ListOptions{PerPage: 100},
		}
		for {
			runnerList, resp, err := client.Actions.ListRunners(
				ctx,
				c.config.GitHub.GetAccount(),
				c.config.GitHub.Repo,
				opts,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to list repo runners: %w", err)
			}

			for _, runner := range runnerList.Runners {
				status := "offline"
				if runner.GetStatus() == "online" {
					status = "online"
				}
				runners = append(runners, RunnerInfo{
					ID:     runner.GetID(),
					Name:   runner.GetName(),
					Status: status,
				})
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
	} else if isUserAccount {
		return nil, fmt.Errorf("personal accounts require a repository to be specified")
	} else {
		// Organization-level runners
		opts := &github.ListRunnersOptions{
			ListOptions: github.ListOptions{PerPage: 100},
		}
		for {
			runnerList, resp, err := client.Actions.ListOrganizationRunners(
				ctx,
				c.config.GitHub.GetAccount(),
				opts,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to list org runners: %w", err)
			}

			for _, runner := range runnerList.Runners {
				status := "offline"
				if runner.GetStatus() == "online" {
					status = "online"
				}
				runners = append(runners, RunnerInfo{
					ID:     runner.GetID(),
					Name:   runner.GetName(),
					Status: status,
				})
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
	}

	c.logger.Debug("Listed runners from GitHub",
		"count", len(runners),
		"repo", c.config.GitHub.Repo)

	return runners, nil
}

// RemoveRunner removes a runner from GitHub by ID
func (c *Client) RemoveRunner(runnerID int64, runnerName string) error {
	// In mock mode, just log
	if c.config.Debug.UseMock {
		c.logger.Debug("Mock mode: skipping runner removal", "runner_id", runnerID, "runner_name", runnerName)
		return nil
	}

	ctx := context.Background()

	client, installation, err := c.getAuthenticatedClient(ctx)
	if err != nil {
		return err
	}

	isUserAccount := installation.Account.GetType() == "User"

	if c.config.GitHub.Repo != "" {
		// Repository-level runner
		resp, err := client.Actions.RemoveRunner(
			ctx,
			c.config.GitHub.GetAccount(),
			c.config.GitHub.Repo,
			runnerID,
		)
		if err != nil {
			return fmt.Errorf("failed to remove repo runner: %w", err)
		}
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("unexpected status code %d when removing runner", resp.StatusCode)
		}
	} else if isUserAccount {
		return fmt.Errorf("personal accounts require a repository to be specified")
	} else {
		// Organization-level runner
		resp, err := client.Actions.RemoveOrganizationRunner(
			ctx,
			c.config.GitHub.GetAccount(),
			runnerID,
		)
		if err != nil {
			return fmt.Errorf("failed to remove org runner: %w", err)
		}
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("unexpected status code %d when removing runner", resp.StatusCode)
		}
	}

	c.logger.Info("Removed runner from GitHub",
		"runner_id", runnerID,
		"runner_name", runnerName)

	return nil
}

// GetRunnerByName finds a specific runner by name
// Returns nil if runner is not found
func (c *Client) GetRunnerByName(name string) (*RunnerInfo, error) {
	runners, err := c.ListRunners()
	if err != nil {
		return nil, fmt.Errorf("failed to list runners: %w", err)
	}

	for _, runner := range runners {
		if runner.Name == name {
			return &runner, nil
		}
	}

	// Runner not found
	return nil, nil
}
