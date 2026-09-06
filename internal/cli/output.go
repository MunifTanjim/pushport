package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// emit: json mode pretty-prints data (nil data, a 204, prints nothing); table
// mode delegates to the renderer.
func emit(cmd *cobra.Command, data json.RawMessage, table func() error) error {
	if flagOutput == "json" {
		if data == nil {
			return nil
		}
		var buf bytes.Buffer
		if err := json.Indent(&buf, data, "", "  "); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), buf.String())
		return nil
	}
	return table()
}

func printTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	_ = tw.Flush()
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func usageErr(msg string) error { return fmt.Errorf("%s", msg) }

func printToken(cmd *cobra.Command, data json.RawMessage) error {
	var v struct {
		Token string `json:"token"`
	}
	if err := dataInto(data, &v); err != nil {
		return err
	}
	printTable(cmd.OutOrStdout(), []string{"TOKEN (shown once)"}, [][]string{{v.Token}})
	return nil
}

func printAppDetail(cmd *cobra.Command, data json.RawMessage) error {
	var a struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		IsPublic      bool   `json:"is_public"`
		KeyVersion    int64  `json:"key_version"`
		MinKeyVersion int64  `json:"min_key_version"`
	}
	if err := dataInto(data, &a); err != nil {
		return err
	}
	printTable(cmd.OutOrStdout(), []string{"ID", "NAME", "PUBLIC", "KEY_VER", "MIN_KEY_VER"},
		[][]string{{a.ID, a.Name, boolStr(a.IsPublic), itoa64(a.KeyVersion), itoa64(a.MinKeyVersion)}})
	return nil
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func intPtr(p *int) string {
	if p == nil {
		return "-"
	}
	return strconv.Itoa(*p)
}

func printPlanID(cmd *cobra.Command, data json.RawMessage) error {
	var p struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
	}
	if err := dataInto(data, &p); err != nil {
		return err
	}
	printTable(cmd.OutOrStdout(), []string{"ID", "NAME", "DEFAULT"}, [][]string{{p.ID, p.Name, boolStr(p.IsDefault)}})
	return nil
}

func mapRows(vals []string) [][]string {
	rows := make([][]string, len(vals))
	for i, v := range vals {
		rows[i] = []string{v}
	}
	return rows
}
