package cli

import "github.com/spf13/cobra"

func newInstanceCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "instance", Short: "Manage an app's instances"}
	cmd.AddCommand(instListCmd(), instGetCmd(), instCreateCmd(), instDeleteCmd(),
		instRotateTokenCmd(), instUpdateCmd())
	return cmd
}

func instListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List instances", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "GET", appPath("/instances"), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error {
				var xs []struct {
					ID, Label   string
					UsagePlanID *string `json:"usage_plan_id"`
				}
				if err := dataInto(data, &xs); err != nil {
					return err
				}
				rows := make([][]string, 0, len(xs))
				for _, x := range xs {
					rows = append(rows, []string{x.ID, x.Label, strPtr(x.UsagePlanID)})
				}
				printTable(cmd.OutOrStdout(), []string{"ID", "LABEL", "USAGE_PLAN"}, rows)
				return nil
			})
		},
	}
}

func instGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get <instance_id>", Short: "Get an instance", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "GET", appPath("/instances/"+args[0]), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error {
				var x struct {
					ID    string `json:"id"`
					AppID string `json:"app_id"`
					Label string `json:"label"`
				}
				if err := dataInto(data, &x); err != nil {
					return err
				}
				printTable(cmd.OutOrStdout(), []string{"ID", "APP", "LABEL"}, [][]string{{x.ID, x.AppID, x.Label}})
				return nil
			})
		},
	}
}

func instCreateCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use: "create", Short: "Create an instance (prints token once)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			body := map[string]any{}
			if cmd.Flags().Changed("label") {
				body["label"] = label
			}
			data, err := newClient().Do(ctx(), "POST", appPath("/instances"), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error {
				var x struct{ ID, Token string }
				if err := dataInto(data, &x); err != nil {
					return err
				}
				printTable(cmd.OutOrStdout(), []string{"ID", "TOKEN (shown once)"}, [][]string{{x.ID, x.Token}})
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "instance label")
	return cmd
}

func instDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use: "delete <instance_id>", Short: "Delete an instance", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "DELETE", appPath("/instances/"+args[0]), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
}

func instRotateTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use: "rotate-token <instance_id>", Short: "Rotate an instance token (prints new token once)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "POST", appPath("/instances/"+args[0]+"/rotate-token"), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return printToken(cmd, data) })
		},
	}
}

func instUpdateCmd() *cobra.Command {
	var label, planID string
	cmd := &cobra.Command{
		Use: "update <instance_id>", Short: "Update an instance's label and/or usage plan", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			body := map[string]any{}
			if cmd.Flags().Changed("label") {
				body["label"] = label
			}
			if cmd.Flags().Changed("usage-plan-id") {
				body["usage_plan_id"] = planID
			}
			if len(body) == 0 {
				return usageErr("nothing to update: pass --label and/or --usage-plan-id")
			}
			data, err := newClient().Do(ctx(), "PATCH", appPath("/instances/"+args[0]), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "new instance label")
	cmd.Flags().StringVar(&planID, "usage-plan-id", "", "assign a usage plan (empty string unassigns)")
	return cmd
}
