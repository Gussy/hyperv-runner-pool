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

// GetRunnerToken generates a GitHub runner registration token using GitHub App authentication
func (c *Client) GetRunnerToken() (string, error) {
	// In mock mode, return a fake token without calling GitHub API
	if c.config.Debug.UseMock {
		mockToken := fmt.Sprintf("mock-runner-token-%d", time.Now().UnixNano())
		c.logger.Debug("Generated mock token", "token", mockToken)
		return mockToken, nil
	}

	ctx := context.Background()

	// Create GitHub App transport with JWT authentication
	appTransport, err := ghinstallation.NewAppsTransportKeyFromFile(
		http.DefaultTransport,
		c.config.GitHub.AppID,
		c.config.GitHub.AppPrivateKeyPath,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	// Create a GitHub client with app authentication to find installation
	appClient := github.NewClient(&http.Client{Transport: appTransport})

	// List all installations for this GitHub App
	// This approach works for both personal accounts and organizations
	installations, _, err := appClient.Apps.ListInstallations(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list installations: %w", err)
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
		return "", fmt.Errorf("GitHub App is not installed on account '%s'. Please install the app at: https://github.com/apps/YOUR_APP_NAME/installations/new", account)
	}

	c.logger.Debug("Found GitHub App installation",
		"installation_id", installation.GetID(),
		"account", account,
		"account_type", installation.Account.GetType())

	// Create installation transport for API calls as this installation
	installationTransport := ghinstallation.NewFromAppsTransport(appTransport, installation.GetID())

	// Create GitHub client authenticated as the installation
	client := github.NewClient(&http.Client{Transport: installationTransport})

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
