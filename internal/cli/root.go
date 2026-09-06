// Package cli is the pushport command-line interface: `serve` runs the server;
// the other commands drive the REST API as an HTTP client.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/MunifTanjim/pushport/internal/cli/client"
)

var (
	flagBaseURL string
	flagToken   string
	flagApp     string
	flagOutput  string
	flagTimeout time.Duration
)

func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:               "pushport",
		Short:             "pushport push-notification relay",
		Version:           version,
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: validateFlags,
	}
	pf := root.PersistentFlags()
	pf.StringVar(&flagBaseURL, "base-url", envOr("PUSHPORT_BASE_URL", "http://localhost:8080"), "pushport server base URL")
	pf.StringVar(&flagToken, "token", firstEnv("PUSHPORT_TOKEN", "PUSHPORT_ADMIN_TOKEN"), "bearer token (admin or app pat_)")
	pf.StringVar(&flagApp, "app", "@app", "app id for app-scoped commands (@app resolves from an app token)")
	pf.StringVarP(&flagOutput, "output", "o", "table", "output format: table|json")
	pf.DurationVar(&flagTimeout, "timeout", 30*time.Second, "HTTP request timeout")

	root.AddCommand(newServeCmd())
	root.AddCommand(newAppCmd())
	root.AddCommand(newCredsCmd())
	root.AddCommand(newInstanceCmd())
	root.AddCommand(newTurnstileCmd())
	root.AddCommand(newUsagePlanCmd())
	root.AddCommand(newSettingsCmd())
	root.AddCommand(newUpgradeCmd())
	return root
}

func validateFlags(_ *cobra.Command, _ []string) error {
	if flagOutput != "table" && flagOutput != "json" {
		return fmt.Errorf("invalid --output %q: must be table or json", flagOutput)
	}
	return nil
}

// Execute runs the CLI and exits non-zero on error.
func Execute(version string) {
	if err := newRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newClient() *client.Client { return client.New(flagBaseURL, flagToken, flagTimeout) }

func appPath(suffix string) string { return "/apps/" + flagApp + suffix }

// requireConcreteApp rejects "@app" when the token can't resolve it: pat_ app
// tokens carry their app id, but the admin token is not bound to one app.
func requireConcreteApp() error {
	if flagApp == "" {
		return fmt.Errorf("this command requires a concrete --app <id>")
	}
	if flagApp == "@app" && !strings.HasPrefix(flagToken, "pat_") {
		return fmt.Errorf("this command requires a concrete --app <id> (the admin token cannot resolve @app)")
	}
	return nil
}

func ctx() context.Context { return context.Background() }

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func dataInto(data json.RawMessage, dst any) error { return json.Unmarshal(data, dst) }
