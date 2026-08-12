// Package ghapp holds the GitHub App plumbing: installation auth, webhook
// intake, and the Check Run lifecycle.
package ghapp

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v89/github"
)

// ClientFactory mints a *github.Client scoped to one installation. A single App
// serves many installations, each with its own short-lived token, so clients
// are built per event rather than once at startup.
type ClientFactory struct {
	privateKey []byte
	appID      int64
}

// NewClientFactory validates the App credentials and returns a factory.
func NewClientFactory(appID int64, privateKeyPEM []byte) (*ClientFactory, error) {
	if appID == 0 {
		return nil, errors.New("ghapp: app ID is required")
	}
	if len(privateKeyPEM) == 0 {
		return nil, errors.New("ghapp: private key is required")
	}
	// Validate the PEM once so a bad key fails at startup, not per webhook.
	if _, err := ghinstallation.NewAppsTransport(http.DefaultTransport, appID, privateKeyPEM); err != nil {
		return nil, fmt.Errorf("ghapp: invalid app private key: %w", err)
	}
	return &ClientFactory{appID: appID, privateKey: privateKeyPEM}, nil
}

// ForInstallation returns a client authenticated as the given installation.
func (f *ClientFactory) ForInstallation(installationID int64) (*github.Client, error) {
	transport, err := ghinstallation.New(http.DefaultTransport, f.appID, installationID, f.privateKey)
	if err != nil {
		return nil, fmt.Errorf("ghapp: build installation transport: %w", err)
	}
	client, err := github.NewClient(github.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		return nil, fmt.Errorf("ghapp: build github client: %w", err)
	}
	return client, nil
}

// InstallationToken returns a fresh installation access token, used to build the
// authenticated clone URL for gitfetch.
func (f *ClientFactory) InstallationToken(ctx context.Context, installationID int64) (string, error) {
	transport, err := ghinstallation.New(http.DefaultTransport, f.appID, installationID, f.privateKey)
	if err != nil {
		return "", fmt.Errorf("ghapp: build installation transport: %w", err)
	}
	tok, err := transport.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("ghapp: mint installation token: %w", err)
	}
	return tok, nil
}
