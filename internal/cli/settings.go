package cli

import "github.com/spf13/cobra"

func newSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "settings", Short: "Manage server settings"}
	cmd.AddCommand(settingsGetCmd(), settingsSetCmd())
	return cmd
}

func settingsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get", Short: "Get server settings", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := newClient().Do(ctx(), "GET", "/settings", nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
}

func settingsSetCmd() *cobra.Command {
	var retryMaxAttempts, authFailIPPerMin, authFailIPBurst int
	var retryBaseBackoff, retryMaxBackoff string
	cmd := &cobra.Command{
		Use: "set", Short: "Update server settings", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			body := map[string]any{}
			if cmd.Flags().Changed("retry-max-attempts") {
				body["retry_max_attempts"] = retryMaxAttempts
			}
			if cmd.Flags().Changed("retry-base-backoff") {
				body["retry_base_backoff"] = retryBaseBackoff
			}
			if cmd.Flags().Changed("retry-max-backoff") {
				body["retry_max_backoff"] = retryMaxBackoff
			}
			if cmd.Flags().Changed("auth-fail-ip-per-min") {
				body["auth_fail_ip_per_min"] = authFailIPPerMin
			}
			if cmd.Flags().Changed("auth-fail-ip-burst") {
				body["auth_fail_ip_burst"] = authFailIPBurst
			}
			if len(body) == 0 {
				return usageErr("nothing to update: pass --retry-* and/or --auth-fail-ip-* flags")
			}
			data, err := newClient().Do(ctx(), "PATCH", "/settings", body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	cmd.Flags().IntVar(&retryMaxAttempts, "retry-max-attempts", 0, "delivery retry max attempts")
	cmd.Flags().StringVar(&retryBaseBackoff, "retry-base-backoff", "", "delivery retry base backoff (Go duration, e.g. 200ms)")
	cmd.Flags().StringVar(&retryMaxBackoff, "retry-max-backoff", "", "delivery retry max backoff (Go duration, e.g. 5s)")
	cmd.Flags().IntVar(&authFailIPPerMin, "auth-fail-ip-per-min", 0, "auth-failure per-IP throttle per-min")
	cmd.Flags().IntVar(&authFailIPBurst, "auth-fail-ip-burst", 0, "auth-failure per-IP throttle burst")
	return cmd
}
