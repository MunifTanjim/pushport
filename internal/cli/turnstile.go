package cli

import "github.com/spf13/cobra"

func newTurnstileCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "turnstile", Short: "Manage per-app Cloudflare Turnstile"}
	cmd.AddCommand(turnstileGetCmd(), turnstileSetCmd(), turnstileRmCmd())
	return cmd
}

func turnstileGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get", Short: "Show the app's Turnstile site key", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "GET", appPath("/turnstile"), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error {
				var v struct {
					SiteKey    *string `json:"site_key"`
					Configured bool    `json:"configured"`
				}
				if err := dataInto(data, &v); err != nil {
					return err
				}
				printTable(cmd.OutOrStdout(), []string{"SITE_KEY", "CONFIGURED"}, [][]string{{strPtr(v.SiteKey), boolStr(v.Configured)}})
				return nil
			})
		},
	}
}

func turnstileSetCmd() *cobra.Command {
	var siteKey, secretKey string
	cmd := &cobra.Command{
		Use: "set", Short: "Set the app's Turnstile keys", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			secret, err := readAtFile(secretKey)
			if err != nil {
				return err
			}
			body := map[string]any{"site_key": siteKey, "secret_key": secret}
			data, err := newClient().Do(ctx(), "PUT", appPath("/turnstile"), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	cmd.Flags().StringVar(&siteKey, "site-key", "", "Turnstile site key")
	cmd.Flags().StringVar(&secretKey, "secret-key", "", "Turnstile secret key (or @file)")
	_ = cmd.MarkFlagRequired("site-key")
	_ = cmd.MarkFlagRequired("secret-key")
	return cmd
}

func turnstileRmCmd() *cobra.Command {
	return &cobra.Command{
		Use: "rm", Short: "Clear the app's Turnstile config", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "DELETE", appPath("/turnstile"), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
}
