package main

import (
	"KernelFactory/builder"
	"KernelFactory/downloader"
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	KernelVersion       string `json:"kernel_version"`
	NvidiaDriverVersion string `json:"nvidia_driver_version"`
	NvidiaSha256        string `json:"nvidia_sha256"`
	RunixOSVersion      string `json:"runixos_version"`
	OutputDir           string `json:"output_dir"`
	BorePatchURL        string `json:"bore_patch_url"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	// Subcommands:
	//   check   - report latest kernel/NVIDIA versions vs config
	//   update  - rewrite config.json with the latest versions
	// Otherwise the first arg, if any, is the config path.
	configPath := "config.json"
	var subcommand string
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "check", "update":
			subcommand = os.Args[1]
		default:
			configPath = os.Args[1]
		}
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		fail("Failed to load %s: %v", configPath, err)
	}

	if subcommand != "" {
		if err := CheckVersions(cfg, configPath, subcommand == "update"); err != nil {
			fail("%v", err)
		}
		return
	}

	fmt.Printf("RunixOS Kernel Factory\n")
	fmt.Printf("  Kernel:  %s\n", cfg.KernelVersion)
	fmt.Printf("  NVIDIA:  %s\n", cfg.NvidiaDriverVersion)
	fmt.Printf("  RunixOS: %s\n", cfg.RunixOSVersion)
	fmt.Printf("  Output:  %s\n", cfg.OutputDir)
	fmt.Println()

	kernelPath, err := downloader.DownloadKernel(cfg.KernelVersion)
	if err != nil {
		fail("Kernel download: %v", err)
	}
	if err := builder.BuildKernel(kernelPath, cfg.KernelVersion, cfg.RunixOSVersion, cfg.OutputDir, cfg.BorePatchURL); err != nil {
		fail("Kernel build: %v", err)
	}

	nvidiaPath, err := downloader.DownloadNvidiaDriver(cfg.NvidiaDriverVersion, cfg.NvidiaSha256)
	if err != nil {
		fail("NVIDIA download: %v", err)
	}
	if err := builder.BuildNvidiaDriver(nvidiaPath, cfg.NvidiaDriverVersion, cfg.KernelVersion, cfg.RunixOSVersion, cfg.OutputDir); err != nil {
		fail("NVIDIA build: %v", err)
	}

	fmt.Println("\nKernel Factory completed successfully.")
}
