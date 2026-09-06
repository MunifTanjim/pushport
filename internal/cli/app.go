package cli

import (
	"github.com/spf13/cobra"
)

func newAppCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "app", Short: "Manage apps"}
	cmd.AddCommand(
		appListCmd(), appCreateCmd(), appGetCmd(), appUpdateCmd(),
		appRotateTokenCmd(), appRotateEndpointKeyCmd(),
	)
	return cmd
}

func appListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List all apps (admin)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := newClient().Do(ctx(), "GET", "/apps", nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error {
				var apps []struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					IsPublic bool   `json:"is_public"`
				}
				if err := dataInto(data, &apps); err != nil {
					return err
				}
				rows := make([][]string, 0, len(apps))
				for _, a := range apps {
					rows = append(rows, []string{a.ID, a.Name, boolStr(a.IsPublic)})
				}
				printTable(cmd.OutOrStdout(), []string{"ID", "NAME", "PUBLIC"}, rows)
				return nil
			})
		},
	}
}

func appCreateCmd() *cobra.Command {
	var name string
	var public bool
	cmd := &cobra.Command{
		Use: "create", Short: "Create an app (admin); prints the app token once", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			body := struct {
				Name     string `json:"name"`
				IsPublic bool   `json:"is_public"`
			}{Name: name, IsPublic: public}
			data, err := newClient().Do(ctx(), "POST", "/apps", body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error {
				var a struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Token string `json:"token"`
				}
				if err := dataInto(data, &a); err != nil {
					return err
				}
				printTable(cmd.OutOrStdout(), []string{"ID", "NAME", "TOKEN (shown once)"}, [][]string{{a.ID, a.Name, a.Token}})
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "app name (required)")
	cmd.Flags().BoolVar(&public, "public", false, "allow public self-registration")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func appGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get", Short: "Get the app (defaults to --app @app)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "GET", appPath(""), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return printAppDetail(cmd, data) })
		},
	}
}

func appUpdateCmd() *cobra.Command {
	var name, planID string
	var public, private bool
	cmd := &cobra.Command{
		Use: "update", Short: "Update the app's name, visibility, and/or usage plan", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			if public && private {
				return usageErr("--public and --private are mutually exclusive")
			}
			body := map[string]any{}
			if cmd.Flags().Changed("name") {
				body["name"] = name
			}
			if public {
				body["is_public"] = true
			}
			if private {
				body["is_public"] = false
			}
			if cmd.Flags().Changed("usage-plan-id") {
				body["usage_plan_id"] = planID
			}
			if len(body) == 0 {
				return usageErr("nothing to update: pass --name, --public|--private, and/or --usage-plan-id")
			}
			data, err := newClient().Do(ctx(), "PATCH", appPath(""), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new app name")
	cmd.Flags().BoolVar(&public, "public", false, "make the app public")
	cmd.Flags().BoolVar(&private, "private", false, "make the app private")
	cmd.Flags().StringVar(&planID, "usage-plan-id", "", "assign a usage plan (admin only; empty string unassigns)")
	return cmd
}

func appRotateTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use: "rotate-token", Short: "Rotate the app's token (prints new token once)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "POST", appPath("/rotate-token"), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return printToken(cmd, data) })
		},
	}
}

func appRotateEndpointKeyCmd() *cobra.Command {
	var revokeOld bool
	cmd := &cobra.Command{
		Use: "rotate-endpoint-key", Short: "Rotate the app's endpoint key", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			body := map[string]any{"revoke_old": revokeOld}
			data, err := newClient().Do(ctx(), "POST", appPath("/rotate-endpoint-key"), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	cmd.Flags().BoolVar(&revokeOld, "revoke-old", false, "immediately revoke endpoints signed with the old key")
	return cmd
}
