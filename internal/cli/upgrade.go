package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

const (
	upgradeRepo       = "MunifTanjim/pushport"
	upgradeBinaryName = "pushport"
)

// upgradeAssetName builds the release-asset filename (<binary>-<tag>-<os>-<arch>).
func upgradeAssetName(tag, goos, goarch string) string {
	return fmt.Sprintf("%s-%s-%s-%s", upgradeBinaryName, tag, goos, goarch)
}

func hasGH() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func latestReleaseTag(ctx context.Context, useGH bool) (string, error) {
	if useGH {
		out, err := exec.CommandContext(ctx, "gh", "release", "view",
			"--repo", upgradeRepo, "--json", "tagName", "--jq", ".tagName").Output()
		if err != nil {
			return "", ghError(err)
		}
		tag := string(trimSpace(out))
		if tag == "" {
			return "", errors.New("release tag missing in response")
		}
		return tag, nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", upgradeRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", errors.New("release tag missing in response")
	}
	return release.TagName, nil
}

// downloadBinary writes the release binary for tag to dst.
func downloadBinary(ctx context.Context, tag, dst string, useGH bool) error {
	assetName := upgradeAssetName(tag, runtime.GOOS, runtime.GOARCH)

	if useGH {
		cmd := exec.CommandContext(ctx, "gh", "release", "download", tag,
			"--repo", upgradeRepo, "--pattern", assetName, "--output", dst, "--clobber")
		if err := cmd.Run(); err != nil {
			return ghError(err)
		}
		return os.Chmod(dst, 0o755)
	}

	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", upgradeRepo, tag, assetName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ghError surfaces the gh CLI's stderr alongside the exit error, when present.
func ghError(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(exit.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, trimSpace(exit.Stderr))
	}
	return err
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ' || b[len(b)-1] == '\t') {
		b = b[:len(b)-1]
	}
	for len(b) > 0 && (b[0] == '\n' || b[0] == '\r' || b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}

// newUpgradeCmd builds `pushport upgrade`: download the latest release binary for
// the current platform and atomically swap it over the running executable.
func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade to the latest version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}
			if execPath, err = filepath.EvalSymlinks(execPath); err != nil {
				return fmt.Errorf("failed to resolve executable path: %w", err)
			}
			// Refuse to swap anything that isn't the installed binary (e.g. `go run`
			// or a renamed copy), so upgrade only ever overwrites a real pushport install.
			if filepath.Base(execPath) != upgradeBinaryName {
				return fmt.Errorf("upgrade can only be run with the %s binary", upgradeBinaryName)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()

			useGH := hasGH()
			tag, err := latestReleaseTag(ctx, useGH)
			if err != nil {
				return fmt.Errorf("failed to resolve latest release: %w", err)
			}

			out := cmd.ErrOrStderr()
			currVersion := cmd.Root().Version
			fmt.Fprintf(out, "Current version: %s\n", currVersion)
			fmt.Fprintf(out, "Latest version: %s\n", tag)

			if tag == currVersion {
				fmt.Fprintln(out, "Already on the latest version!")
				return nil
			}

			// Keep the temp file beside the target so the final rename stays on the
			// same filesystem (atomic, and safe to swap a running binary on Unix).
			tempPath := execPath + ".tmp"

			fmt.Fprintf(out, "Downloading %s...\n", upgradeAssetName(tag, runtime.GOOS, runtime.GOARCH))
			if err := downloadBinary(ctx, tag, tempPath, useGH); err != nil {
				os.Remove(tempPath)
				return fmt.Errorf("failed to download: %w", err)
			}

			fmt.Fprintln(out, "Verifying downloaded binary...")
			if err := exec.CommandContext(ctx, tempPath, "--version").Run(); err != nil {
				os.Remove(tempPath)
				return fmt.Errorf("downloaded binary verification failed: %w", err)
			}

			if err := os.Rename(tempPath, execPath); err != nil {
				os.Remove(tempPath)
				return fmt.Errorf("failed to replace binary: %w", err)
			}

			fmt.Fprintf(out, "Upgraded to %s!\n", tag)
			return nil
		},
	}
}
