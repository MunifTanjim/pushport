package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func readAtFile(v string) (string, error) {
	if strings.HasPrefix(v, "@") {
		b, err := os.ReadFile(v[1:])
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return v, nil
}

func newCredsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "creds", Short: "Manage an app's push credentials"}
	cmd.AddCommand(credsListCmd(), credsSetCmd(), credsRmCmd())
	return cmd
}

func credsListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List configured transports", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "GET", appPath("/creds"), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error {
				var v struct{ Transports []string }
				if err := dataInto(data, &v); err != nil {
					return err
				}
				printTable(cmd.OutOrStdout(), []string{"TRANSPORT"}, mapRows(v.Transports))
				return nil
			})
		},
	}
}

func credsSetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "set", Short: "Set credentials for a transport"}
	cmd.AddCommand(credsSetApnsCmd(), credsSetFCMCmd(), credsSetWebPushCmd())
	return cmd
}

func credsSetApnsCmd() *cobra.Command {
	var keyP8, keyID, teamID, topic string
	cmd := &cobra.Command{
		Use: "apns", Short: "Set APNs credentials", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			p8, err := readAtFile(keyP8)
			if err != nil {
				return err
			}
			body := map[string]any{"key_p8": p8, "key_id": keyID, "team_id": teamID, "topic": topic}
			data, err := newClient().Do(ctx(), "PUT", appPath("/creds/apns"), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	f := cmd.Flags()
	f.StringVar(&keyP8, "key-p8", "", "APNs .p8 key PEM (or @file)")
	f.StringVar(&keyID, "key-id", "", "APNs key id")
	f.StringVar(&teamID, "team-id", "", "Apple team id")
	f.StringVar(&topic, "topic", "", "APNs topic (bundle id)")
	for _, n := range []string{"key-p8", "key-id", "team-id", "topic"} {
		_ = cmd.MarkFlagRequired(n)
	}
	return cmd
}

func credsSetFCMCmd() *cobra.Command {
	var serviceAccount string
	cmd := &cobra.Command{
		Use: "fcm", Short: "Set FCM credentials", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			sa, err := readAtFile(serviceAccount)
			if err != nil {
				return err
			}
			body := map[string]any{"service_account_json": sa}
			data, err := newClient().Do(ctx(), "PUT", appPath("/creds/fcm"), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	f := cmd.Flags()
	f.StringVar(&serviceAccount, "service-account", "", "FCM service-account JSON (or @file)")
	_ = cmd.MarkFlagRequired("service-account")
	return cmd
}

func credsSetWebPushCmd() *cobra.Command {
	var priv, subject string
	cmd := &cobra.Command{
		Use: "webpush", Short: "Set WebPush (VAPID) credentials", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			privVal, err := readAtFile(priv)
			if err != nil {
				return err
			}
			body := map[string]any{"vapid_private_key": privVal, "subject": subject}
			data, err := newClient().Do(ctx(), "PUT", appPath("/creds/webpush"), body)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
	f := cmd.Flags()
	f.StringVar(&priv, "vapid-private", "", "VAPID private key (or @file)")
	f.StringVar(&subject, "subject", "", "VAPID subject (mailto: or URL)")
	for _, n := range []string{"vapid-private", "subject"} {
		_ = cmd.MarkFlagRequired(n)
	}
	return cmd
}

func credsRmCmd() *cobra.Command {
	return &cobra.Command{
		Use: "rm <apns|fcm|webpush>", Short: "Remove a transport's credentials",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"apns", "fcm", "webpush"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConcreteApp(); err != nil {
				return err
			}
			data, err := newClient().Do(ctx(), "DELETE", appPath("/creds/"+args[0]), nil)
			if err != nil {
				return err
			}
			return emit(cmd, data, func() error { return nil })
		},
	}
}
