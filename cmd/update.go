package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const heimdalRepo = "coone-ai/heimdal"

type githubRelease struct {
	TagName string `json:"tag_name"`
}

var updateCmd = &cobra.Command{
	Use:     "update",
	Short:   "Update Heimdal CLI to the latest release",
	GroupID: "session",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		latest, err := LatestReleaseTag(ctx)
		if err != nil {
			return err
		}
		if latest == "" {
			return fmt.Errorf("failed to resolve latest release")
		}
		if !IsNewerVersion(Version, latest) {
			fmt.Printf("heimdal is already up to date (%s)\n", DisplayVersion())
			return nil
		}
		if runtime.GOOS == "windows" {
			fmt.Println("Update available:", latest)
			fmt.Println("On Windows, run:")
			fmt.Println("  irm https://raw.githubusercontent.com/coone-ai/heimdal/main/scripts/install.ps1 | iex")
			return nil
		}

		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to locate current executable: %w", err)
		}
		if err := installRelease(ctx, latest, exe); err != nil {
			return err
		}
		fmt.Printf("Updated heimdal: %s -> %s\n", DisplayVersion(), latest)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func DisplayVersion() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		return "dev"
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	if v == "dev" {
		return v
	}
	return "v" + v
}

func LatestReleaseTag(ctx context.Context) (string, error) {
	if tag, err := latestReleaseTagFromAPI(ctx); err == nil && tag != "" {
		return tag, nil
	}
	return latestReleaseTagFromRedirect(ctx)
}

func latestReleaseTagFromAPI(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+heimdalRepo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "heimdal-cli/"+DisplayVersion())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to check latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("failed to check latest release: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse latest release: %w", err)
	}
	return strings.TrimSpace(release.TagName), nil
}

func latestReleaseTagFromRedirect(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/"+heimdalRepo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "heimdal-cli/"+DisplayVersion())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to check latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("failed to check latest release: HTTP %d", resp.StatusCode)
	}

	parts := strings.Split(strings.Trim(resp.Request.URL.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "tag" {
			return strings.TrimSpace(parts[i+1]), nil
		}
	}
	return "", fmt.Errorf("failed to resolve latest release from %s", resp.Request.URL.String())
}

func IsNewerVersion(current, latest string) bool {
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")
	latest = strings.TrimPrefix(strings.TrimSpace(latest), "v")
	if current == "" || current == "dev" || latest == "" {
		return false
	}
	cv := versionParts(current)
	lv := versionParts(latest)
	for i := 0; i < 3; i++ {
		if lv[i] > cv[i] {
			return true
		}
		if lv[i] < cv[i] {
			return false
		}
	}
	return false
}

func versionParts(v string) [3]int {
	main := strings.SplitN(v, "-", 2)[0]
	parts := strings.Split(main, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

func installRelease(ctx context.Context, tag, target string) error {
	assetVersion := strings.TrimPrefix(tag, "v")
	archiveExt := ".tar.gz"
	if runtime.GOOS == "windows" {
		archiveExt = ".zip"
	}
	archiveName := fmt.Sprintf("heimdal_%s_%s_%s%s", assetVersion, runtime.GOOS, runtime.GOARCH, archiveExt)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", heimdalRepo, tag, archiveName)

	tmpDir, err := os.MkdirTemp("", "heimdal-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName)
	if err := downloadFile(ctx, url, archivePath); err != nil {
		return err
	}
	extractedPath, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		return err
	}
	if err := os.Chmod(extractedPath, 0o755); err != nil {
		return err
	}

	backup := target + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("failed to replace %s: %w\nTry rerunning the installer with sufficient permissions", target, err)
	}
	if err := os.Rename(extractedPath, target); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("failed to install update: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}

func downloadFile(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "heimdal-cli/"+DisplayVersion())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to download update: HTTP %d: %s", resp.StatusCode, url)
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(archivePath, destDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZipBinary(archivePath, destDir)
	}
	return extractTarGzBinary(archivePath, destDir)
}

func extractTarGzBinary(archivePath, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.FileInfo().IsDir() || filepath.Base(header.Name) != "heimdal" {
			continue
		}
		outPath := filepath.Join(destDir, "heimdal")
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return "", err
		}
		out.Close()
		return outPath, nil
	}
	return "", fmt.Errorf("heimdal binary not found in archive")
}

func extractZipBinary(archivePath, destDir string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, file := range zr.File {
		if file.FileInfo().IsDir() || filepath.Base(file.Name) != "heimdal.exe" {
			continue
		}
		in, err := file.Open()
		if err != nil {
			return "", err
		}
		defer in.Close()
		outPath := filepath.Join(destDir, "heimdal.exe")
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return "", err
		}
		out.Close()
		return outPath, nil
	}
	return "", fmt.Errorf("heimdal.exe not found in archive")
}
