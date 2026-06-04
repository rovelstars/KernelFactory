package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// getJSON fetches url and decodes the JSON body into v. A User-Agent is set so
// the GitHub API does not reject the request.
func getJSON(url string, v any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "KernelFactory")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// latestKernel returns kernel.org's current stable version (e.g. "7.0.11").
func latestKernel() (string, error) {
	var data struct {
		LatestStable struct {
			Version string `json:"version"`
		} `json:"latest_stable"`
	}
	if err := getJSON("https://www.kernel.org/releases.json", &data); err != nil {
		return "", err
	}
	if data.LatestStable.Version == "" {
		return "", fmt.Errorf("no latest_stable in releases.json")
	}
	return data.LatestStable.Version, nil
}

// latestNvidia returns the latest open-gpu-kernel-modules release tag.
func latestNvidia() (string, error) {
	var data struct {
		TagName string `json:"tag_name"`
	}
	url := "https://api.github.com/repos/NVIDIA/open-gpu-kernel-modules/releases/latest"
	if err := getJSON(url, &data); err != nil {
		return "", err
	}
	if data.TagName == "" {
		return "", fmt.Errorf("no tag_name in NVIDIA release")
	}
	return data.TagName, nil
}

// borePatchURL finds the BORE patch matching a kernel's major.minor series.
func borePatchURL(kernelVersion string) (string, error) {
	parts := strings.Split(kernelVersion, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("bad kernel version %q", kernelVersion)
	}
	majmin := parts[0] + "." + parts[1]
	dir := "patches/stable/linux-" + majmin + "-bore"

	var entries []struct {
		Name string `json:"name"`
	}
	api := "https://api.github.com/repos/firelzrd/bore-scheduler/contents/" + dir
	if err := getJSON(api, &entries); err != nil {
		return "", fmt.Errorf("no BORE patch for linux-%s: %w", majmin, err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name, ".patch") {
			return fmt.Sprintf("https://raw.githubusercontent.com/firelzrd/bore-scheduler/refs/heads/main/%s/%s", dir, e.Name), nil
		}
	}
	return "", fmt.Errorf("no .patch file in %s", dir)
}

// CheckVersions reports the latest kernel and NVIDIA versions against the
// config. When update is true it rewrites configPath with the new values
// (kernel, NVIDIA, and the matching BORE patch URL; nvidia_sha256 is cleared
// so it gets re-pinned for the new tarball).
func CheckVersions(cfg *Config, configPath string, update bool) error {
	kernel, err := latestKernel()
	if err != nil {
		return fmt.Errorf("check kernel: %w", err)
	}
	nvidia, err := latestNvidia()
	if err != nil {
		return fmt.Errorf("check nvidia: %w", err)
	}

	mark := func(cur, latest string) string {
		if cur == latest {
			return "up to date"
		}
		return "UPDATE -> " + latest
	}
	fmt.Printf("Kernel:  %-12s %s\n", cfg.KernelVersion, mark(cfg.KernelVersion, kernel))
	fmt.Printf("NVIDIA:  %-12s %s\n", cfg.NvidiaDriverVersion, mark(cfg.NvidiaDriverVersion, nvidia))

	if !update {
		fmt.Println("\nRun `go run . update` to apply.")
		return nil
	}

	bore, err := borePatchURL(kernel)
	if err != nil {
		return fmt.Errorf("find BORE patch: %w", err)
	}
	cfg.KernelVersion = kernel
	cfg.NvidiaDriverVersion = nvidia
	cfg.NvidiaSha256 = ""
	cfg.BorePatchURL = bore

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, append(out, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("\nUpdated %s (kernel %s, nvidia %s, BORE patch set; pin nvidia_sha256 manually).\n", configPath, kernel, nvidia)
	return nil
}
