package cli

import "github.com/spf13/cobra"

func newUsagePlanCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "usage-plan", Short: "Manage usage plans"}
	cmd.AddCommand(usagePlanAppCmd(), usagePlanInstanceCmd())
	return cmd
}

func usagePlanAppCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "app", Short: "Admin app usage plans"}
	cmd.AddCommand(upAppListCmd(), upAppCreateCmd(), upAppGetCmd(), upAppUpdateCmd(), upAppDeleteCmd())
	return cmd
}

func upAppListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List app plans", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := newClient().Do(ctx(), "GET", "/usage-plans", nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error {
				var xs []struct {
					ID             string  `json:"id"`
					AppID          *string `json:"app_id"`
					Name           string  `json:"name"`
					IsDefault      bool    `json:"is_default"`
					PushPerMin     *int    `json:"push_per_min"`
					PushDailyQuota *int    `json:"push_daily_quota"`
				}
				if err := dataInto(data, &xs); err != nil {
					return err
				}
				rows := make([][]string, 0, len(xs))
				for _, x := range xs {
					rows = append(rows, []string{x.ID, strPtr(x.AppID), x.Name, boolStr(x.IsDefault), intPtr(x.PushPerMin), intPtr(x.PushDailyQuota)})
				}
				printTable(cmd.OutOrStdout(), []string{"ID", "APP", "NAME", "DEFAULT", "PER_MIN", "DAILY_QUOTA"}, rows)
				return nil
			})
		},
	}
}

func upAppCreateCmd() *cobra.Command {
	var name, appID string
	var perMinute, dailyQuota int
	var registerPerMin, registerBurst, registerIPPerMin, registerIPBurst, subscribePerMin, subscribeBurst int
	cmd := &cobra.Command{
		Use: "create", Short: "Create an app plan", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			body := map[string]any{"name": name}
			if cmd.Flags().Changed("app-id") {
				body["app_id"] = appID
			}
			if cmd.Flags().Changed("per-min") {
				body["push_per_min"] = perMinute
			}
			if cmd.Flags().Changed("daily-quota") {
				body["push_daily_quota"] = dailyQuota
			}
			if cmd.Flags().Changed("register-per-min") {
				body["register_per_min"] = registerPerMin
			}
			if cmd.Flags().Changed("register-burst") {
				body["register_burst"] = registerBurst
			}
			if cmd.Flags().Changed("register-ip-per-min") {
				body["register_ip_per_min"] = registerIPPerMin
			}
			if cmd.Flags().Changed("register-ip-burst") {
				body["register_ip_burst"] = registerIPBurst
			}
			if cmd.Flags().Changed("subscribe-per-min") {
				body["subscribe_per_min"] = subscribePerMin
			}
			if cmd.Flags().Changed("subscribe-burst") {
				body["subscribe_burst"] = subscribeBurst
			}
			data, err := newClient().Do(ctx(), "POST", "/usage-plans", body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return printPlanID(cmd, data) })
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "plan name (required)")
	cmd.Flags().StringVar(&appID, "app-id", "", "scope the plan to this app only (bespoke)")
	cmd.Flags().IntVar(&perMinute, "per-min", 0, "app push per-min limit")
	cmd.Flags().IntVar(&dailyQuota, "daily-quota", 0, "app push daily quota")
	cmd.Flags().IntVar(&registerPerMin, "register-per-min", 0, "self-registration per-min limit")
	cmd.Flags().IntVar(&registerBurst, "register-burst", 0, "self-registration burst limit")
	cmd.Flags().IntVar(&registerIPPerMin, "register-ip-per-min", 0, "self-registration per-IP per-min limit")
	cmd.Flags().IntVar(&registerIPBurst, "register-ip-burst", 0, "self-registration per-IP burst limit")
	cmd.Flags().IntVar(&subscribePerMin, "subscribe-per-min", 0, "subscribe per-min limit")
	cmd.Flags().IntVar(&subscribeBurst, "subscribe-burst", 0, "subscribe burst limit")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("per-min")
	_ = cmd.MarkFlagRequired("daily-quota")
	return cmd
}

func upAppGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get <plan_id>", Short: "Get an app plan", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := newClient().Do(ctx(), "GET", "/usage-plans/"+args[0], nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return printPlanID(cmd, data) })
		},
	}
}

func upAppUpdateCmd() *cobra.Command {
	var name string
	var perMinute, dailyQuota int
	var registerPerMin, registerBurst, registerIPPerMin, registerIPBurst, subscribePerMin, subscribeBurst int
	cmd := &cobra.Command{
		Use: "update <plan_id>", Short: "Update an app plan", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if cmd.Flags().Changed("name") {
				body["name"] = name
			}
			if cmd.Flags().Changed("per-min") {
				body["push_per_min"] = perMinute
			}
			if cmd.Flags().Changed("daily-quota") {
				body["push_daily_quota"] = dailyQuota
			}
			if cmd.Flags().Changed("register-per-min") {
				body["register_per_min"] = registerPerMin
			}
			if cmd.Flags().Changed("register-burst") {
				body["register_burst"] = registerBurst
			}
			if cmd.Flags().Changed("register-ip-per-min") {
				body["register_ip_per_min"] = registerIPPerMin
			}
			if cmd.Flags().Changed("register-ip-burst") {
				body["register_ip_burst"] = registerIPBurst
			}
			if cmd.Flags().Changed("subscribe-per-min") {
				body["subscribe_per_min"] = subscribePerMin
			}
			if cmd.Flags().Changed("subscribe-burst") {
				body["subscribe_burst"] = subscribeBurst
			}
			if len(body) == 0 {
				return usageErr("nothing to update: pass --name, --per-min, --daily-quota, --register-*, and/or --subscribe-* flags")
			}
			data, err := newClient().Do(ctx(), "PATCH", "/usage-plans/"+args[0], body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new plan name")
	cmd.Flags().IntVar(&perMinute, "per-min", 0, "app push per-min limit")
	cmd.Flags().IntVar(&dailyQuota, "daily-quota", 0, "app push daily quota")
	cmd.Flags().IntVar(&registerPerMin, "register-per-min", 0, "self-registration per-min limit")
	cmd.Flags().IntVar(&registerBurst, "register-burst", 0, "self-registration burst limit")
	cmd.Flags().IntVar(&registerIPPerMin, "register-ip-per-min", 0, "self-registration per-IP per-min limit")
	cmd.Flags().IntVar(&registerIPBurst, "register-ip-burst", 0, "self-registration per-IP burst limit")
	cmd.Flags().IntVar(&subscribePerMin, "subscribe-per-min", 0, "subscribe per-min limit")
	cmd.Flags().IntVar(&subscribeBurst, "subscribe-burst", 0, "subscribe burst limit")
	return cmd
}

func upAppDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use: "delete <plan_id>", Short: "Delete an app plan", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := newClient().Do(ctx(), "DELETE", "/usage-plans/"+args[0], nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
}

func usagePlanInstanceCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "instance", Short: "Instance usage plans"}
	cmd.AddCommand(upInstListCmd(), upInstCreateCmd(), upInstGetCmd(), upInstUpdateCmd(), upInstDeleteCmd())
	return cmd
}

func upInstListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List instance plans", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "GET", appPath("/usage-plans"), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error {
				var xs []struct {
					ID             string  `json:"id"`
					InstanceID     *string `json:"instance_id"`
					Name           string  `json:"name"`
					IsDefault      bool    `json:"is_default"`
					PushPerMin     *int    `json:"push_per_min"`
					PushBurst      *int    `json:"push_burst"`
					PushDailyQuota *int    `json:"push_daily_quota"`
				}
				if err := dataInto(data, &xs); err != nil {
					return err
				}
				rows := make([][]string, 0, len(xs))
				for _, x := range xs {
					rows = append(rows, []string{x.ID, strPtr(x.InstanceID), x.Name, boolStr(x.IsDefault), intPtr(x.PushPerMin), intPtr(x.PushBurst), intPtr(x.PushDailyQuota)})
				}
				printTable(cmd.OutOrStdout(), []string{"ID", "INSTANCE", "NAME", "DEFAULT", "PER_MIN", "BURST", "DAILY_QUOTA"}, rows)
				return nil
			})
		},
	}
}

func upInstCreateCmd() *cobra.Command {
	var name, instanceID string
	var perMinute, burst, dailyQuota int
	cmd := &cobra.Command{
		Use: "create", Short: "Create an instance plan", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			body := map[string]any{"name": name}
			if cmd.Flags().Changed("instance-id") {
				body["instance_id"] = instanceID
			}
			if cmd.Flags().Changed("per-min") {
				body["push_per_min"] = perMinute
			}
			if cmd.Flags().Changed("burst") {
				body["push_burst"] = burst
			}
			if cmd.Flags().Changed("daily-quota") {
				body["push_daily_quota"] = dailyQuota
			}
			data, err := newClient().Do(ctx(), "POST", appPath("/usage-plans"), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return printPlanID(cmd, data) })
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "plan name (required)")
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "scope the plan to this instance only (bespoke)")
	cmd.Flags().IntVar(&perMinute, "per-min", 0, "instance push per-min limit")
	cmd.Flags().IntVar(&burst, "burst", 0, "instance push burst")
	cmd.Flags().IntVar(&dailyQuota, "daily-quota", 0, "instance push daily quota")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("per-min")
	_ = cmd.MarkFlagRequired("burst")
	_ = cmd.MarkFlagRequired("daily-quota")
	return cmd
}

func upInstGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get <plan_id>", Short: "Get an instance plan", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "GET", appPath("/usage-plans/"+args[0]), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return printPlanID(cmd, data) })
		},
	}
}

func upInstUpdateCmd() *cobra.Command {
	var name string
	var perMinute, burst, dailyQuota int
	cmd := &cobra.Command{
		Use: "update <plan_id>", Short: "Update an instance plan (also tunes the default plan)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			body := map[string]any{}
			if cmd.Flags().Changed("name") {
				body["name"] = name
			}
			if cmd.Flags().Changed("per-min") {
				body["push_per_min"] = perMinute
			}
			if cmd.Flags().Changed("burst") {
				body["push_burst"] = burst
			}
			if cmd.Flags().Changed("daily-quota") {
				body["push_daily_quota"] = dailyQuota
			}
			if len(body) == 0 {
				return usageErr("nothing to update: pass --name, --per-min, --burst, and/or --daily-quota")
			}
			data, err := newClient().Do(ctx(), "PATCH", appPath("/usage-plans/"+args[0]), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new plan name")
	cmd.Flags().IntVar(&perMinute, "per-min", 0, "instance push per-min limit")
	cmd.Flags().IntVar(&burst, "burst", 0, "instance push burst")
	cmd.Flags().IntVar(&dailyQuota, "daily-quota", 0, "instance push daily quota")
	return cmd
}

func upInstDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use: "delete <plan_id>", Short: "Delete an instance plan", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "DELETE", appPath("/usage-plans/"+args[0]), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
}
