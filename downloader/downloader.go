package downloader

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// download fetches url into path unless path already exists.
func download(url, path string) error {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("%s already downloaded.\n", path)
		return nil
	}
	fmt.Println("Downloading from:", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	fmt.Println("Downloaded:", path)
	return nil
}

// sha256File returns the lowercase hex sha256 of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifySha256(path, expected string) error {
	got, err := sha256File(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", path, got, expected)
	}
	fmt.Printf("Verified sha256 of %s\n", path)
	return nil
}

// DownloadKernel downloads the kernel tarball and verifies its sha256 against
// kernel.org's published sums for that release series. Returns the local path.
//
// Note: this trusts the sums file over TLS (the .asc is PGP-signed upstream; we
// do not yet verify that signature locally - a future hardening would import
// the kernel.org keys and gpg --verify the sums file).
func DownloadKernel(version string) (string, error) {
	major := strings.SplitN(version, ".", 2)[0]
	url := fmt.Sprintf("https://cdn.kernel.org/pub/linux/kernel/v%s.x/linux-%s.tar.xz", major, version)
	path := fmt.Sprintf("linux-%s.tar.xz", version)
	if err := download(url, path); err != nil {
		return "", err
	}

	sumsURL := fmt.Sprintf("https://cdn.kernel.org/pub/linux/kernel/v%s.x/sha256sums.asc", major)
	expected, err := fetchKernelSha256(sumsURL, path)
	if err != nil {
		return "", fmt.Errorf("kernel integrity: %w", err)
	}
	if err := verifySha256(path, expected); err != nil {
		return "", err
	}
	return path, nil
}

// fetchKernelSha256 downloads kernel.org's sha256sums.asc and returns the
// expected hash for the given tarball file name.
func fetchKernelSha256(sumsURL, tarPath string) (string, error) {
	resp, err := http.Get(sumsURL)
	if err != nil {
		return "", fmt.Errorf("get sums %s: %w", sumsURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get sums %s: status %d", sumsURL, resp.StatusCode)
	}
	want := tarPath // sums list bare file names
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == want {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no sha256 entry for %s in %s", want, sumsURL)
}

// DownloadNvidiaDriver downloads the NVIDIA open-gpu-kernel-modules tarball.
// If expectedSha256 is non-empty it is verified; otherwise a warning is logged.
// Returns the local path.
func DownloadNvidiaDriver(version, expectedSha256 string) (string, error) {
	url := fmt.Sprintf("https://github.com/NVIDIA/open-gpu-kernel-modules/archive/refs/tags/%s.tar.gz", version)
	path := fmt.Sprintf("nvidia-driver-%s.tar.gz", version)
	if err := download(url, path); err != nil {
		return "", err
	}
	if expectedSha256 != "" {
		if err := verifySha256(path, expectedSha256); err != nil {
			return "", err
		}
	} else {
		fmt.Println("Warning: no nvidia_sha256 in config; skipping NVIDIA integrity check")
	}
	return path, nil
}
