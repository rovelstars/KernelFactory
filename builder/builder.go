package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"KernelFactory/utils"
)

// kernelRelease asks the kernel build for its true release string (VERSION +
// localversion), e.g. "6.17.0-runixos-26.2". This is the single source of
// truth for module directory names, so kernel and NVIDIA modules always agree.
// Must be called after the config is in place.
func kernelRelease(kernelSrc string) (string, error) {
	out, err := exec.Command("make", "-s", "-C", kernelSrc, "kernelrelease").Output()
	if err != nil {
		return "", fmt.Errorf("make kernelrelease: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// reproducibleEnv returns kbuild env vars that make the build deterministic.
func reproducibleEnv() []string {
	env := []string{
		"KBUILD_BUILD_USER=runixos",
		"KBUILD_BUILD_HOST=rovelstars",
	}
	if sde := os.Getenv("SOURCE_DATE_EPOCH"); sde != "" {
		if n, err := strconv.ParseInt(sde, 10, 64); err == nil {
			ts := time.Unix(n, 0).UTC().Format("Mon Jan 2 15:04:05 UTC 2006")
			env = append(env, "KBUILD_BUILD_TIMESTAMP="+ts)
		}
	}
	return env
}

func destDir(fallbackSrc string) string {
	if d := os.Getenv("DESTDIR"); d != "" {
		return d
	}
	return fallbackSrc
}

// BuildKernel extracts, patches, configures, builds and installs the kernel.
// borePatchURL is optional; when empty the BORE scheduler patch is skipped.
func BuildKernel(kernelPath, kernelVersion, runixosVersion, src, borePatchURL string) error {
	fmt.Println("Building kernel...")
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("resolve src: %w", err)
	}

	fmt.Printf("Extracting %s to %s\n", kernelPath, absSrc)
	if err := utils.ExtractTar(kernelPath, absSrc); err != nil {
		return fmt.Errorf("extract kernel: %w", err)
	}

	kernelSrc := filepath.Join(absSrc, "linux-"+kernelVersion)

	localVersion := fmt.Sprintf("-runixos-%s", runixosVersion)
	if err := SetLocalVersion(kernelSrc, localVersion); err != nil {
		return fmt.Errorf("set local version: %w", err)
	}

	if borePatchURL != "" {
		if err := applyBorePatch(kernelSrc, borePatchURL); err != nil {
			return fmt.Errorf("bore patch: %w", err)
		}
	} else {
		fmt.Println("No bore_patch_url in config; skipping BORE patch")
	}

	buildEnv := reproducibleEnv()

	// Config: start from defconfig, merge our fragment, normalise.
	if err := utils.Make(kernelSrc, "defconfig", buildEnv...); err != nil {
		return err
	}
	mergeScript := filepath.Join(kernelSrc, "scripts/kconfig/merge_config.sh")
	if err := utils.Run(kernelSrc, mergeScript, []string{"-m", ".config", "THECONFIG"}, buildEnv...); err != nil {
		return fmt.Errorf("merge config: %w", err)
	}
	if err := utils.Make(kernelSrc, "olddefconfig", buildEnv...); err != nil {
		return err
	}

	// Build image + modules.
	if err := utils.Make(kernelSrc, "bzImage modules", buildEnv...); err != nil {
		return err
	}

	release, err := kernelRelease(kernelSrc)
	if err != nil {
		return err
	}
	fmt.Printf("Kernel release: %s\n", release)

	dst := destDir(absSrc)

	// Kernel image -> Core/Startup
	if err := utils.Make(kernelSrc, "install",
		append(buildEnv, "INSTALL_PATH="+dst+"/Core/Startup")...); err != nil {
		return err
	}

	// Modules -> Core/LibKit/modules/<release>. MODLIB is set explicitly so
	// modules land directly there (kbuild would otherwise append lib/modules),
	// and so NVIDIA modules install alongside the kernel's.
	modlib := fmt.Sprintf("%s/Core/LibKit/modules/%s", dst, release)
	if err := utils.Make(kernelSrc, "modules_install",
		append(buildEnv, "MODLIB="+modlib)...); err != nil {
		return err
	}

	// UAPI headers -> Core/APIHeader (all of include/, not just a few dirs).
	if err := installHeaders(kernelSrc, dst, buildEnv); err != nil {
		return err
	}

	return writeRunixOSRelease(dst, runixosVersion, kernelVersion, release)
}

func applyBorePatch(kernelSrc, url string) error {
	patchPath := filepath.Join(kernelSrc, "bore.patch")
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		if err := utils.Run("", "wget", []string{"-O", patchPath, url}); err != nil {
			return fmt.Errorf("download bore patch: %w", err)
		}
	}
	if err := utils.Run(kernelSrc, "patch", []string{"-p1", "-i", patchPath}); err != nil {
		return fmt.Errorf("apply bore patch: %w", err)
	}
	fmt.Println("BORE patch applied")
	return nil
}

func installHeaders(kernelSrc, dst string, buildEnv []string) error {
	hdrTmp := filepath.Join(dst, "Core/.hdrtmp")
	if err := utils.Make(kernelSrc, "headers_install",
		append(buildEnv, "INSTALL_HDR_PATH="+hdrTmp)...); err != nil {
		return err
	}
	includeDir := filepath.Join(hdrTmp, "include")
	entries, err := os.ReadDir(includeDir)
	if err != nil {
		return fmt.Errorf("read kernel headers: %w", err)
	}
	apiHeader := filepath.Join(dst, "Core/APIHeader")
	if err := os.MkdirAll(apiHeader, 0755); err != nil {
		return err
	}
	for _, e := range entries {
		from := filepath.Join(includeDir, e.Name())
		to := filepath.Join(apiHeader, e.Name())
		os.RemoveAll(to)
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("move kernel headers %s: %w", e.Name(), err)
		}
	}
	os.RemoveAll(hdrTmp)
	return nil
}

// writeRunixOSRelease creates /Core/Config/OSReleaseInfo.
func writeRunixOSRelease(dst, runixosVersion, kernelVersion, release string) error {
	configDir := filepath.Join(dst, "Core/Config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	content := fmt.Sprintf(`NAME="RunixOS"
ID=runixos
VERSION=%s
PRETTY_NAME="RunixOS %s"
HOME_URL="https://os.rovelstars.com"
BUG_REPORT_URL="https://os.rovelstars.com/bugreport"
VENDOR="RovelStars"
KERNEL_VERSION=%s
KERNEL_RELEASE=%s
`, runixosVersion, runixosVersion, kernelVersion, release)

	path := filepath.Join(configDir, "OSReleaseInfo")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write OSReleaseInfo: %w", err)
	}
	fmt.Printf("Created %s\n", path)
	return nil
}

func SetLocalVersion(src, ver string) error {
	if err := os.WriteFile(filepath.Join(src, "localversion"), []byte(ver), 0644); err != nil {
		return fmt.Errorf("write localversion: %w", err)
	}
	if err := os.WriteFile(filepath.Join(src, "THECONFIG"), []byte(theConfig), 0644); err != nil {
		return fmt.Errorf("write THECONFIG: %w", err)
	}
	return nil
}

// theConfig is the RunixOS kernel config fragment merged on top of defconfig.
const theConfig = `# === Timer & Preemption (Hybrid Tickless - Windows like) ===
CONFIG_HZ_250=y                # Base tick: 250Hz (4ms) balanced
CONFIG_HZ=250
CONFIG_PREEMPT_VOLUNTARY=y     # Like Windows, preempt but not fully RT
CONFIG_NO_HZ_IDLE=y            # Tickless when CPU is idle (saves power)
CONFIG_HIGH_RES_TIMERS=y       # Apps can request sub-ms precision
CONFIG_SCHED_MC=y              # Multi-core aware scheduler
CONFIG_SCHED_SMT=y             # SMT/Hyper-Threading aware

# === CPU & Scheduler ===
CONFIG_CPU_FREQ=y
CONFIG_CPU_FREQ_GOV_PERFORMANCE=y
CONFIG_CFS_BANDWIDTH=y
CONFIG_CPU_IDLE=y
CONFIG_CPU_IDLE_GOV_LADDER=y
CONFIG_X86_INTEL_MEMORY_PROTECTION_KEYS=y
CONFIG_MICROCODE=y

# === Memory Management ===
CONFIG_ZRAM=y
CONFIG_ZSWAP=y
CONFIG_TRANSPARENT_HUGEPAGE=y
CONFIG_HUGETLBFS=y
CONFIG_MEMCG=y

# === Filesystems ===
CONFIG_EXT4_FS=y
CONFIG_EXT4_USE_FOR_EXT2=y
CONFIG_BTRFS_FS=y
CONFIG_F2FS_FS=y
CONFIG_EROFS_FS=y
CONFIG_SQUASHFS=y
CONFIG_SQUASHFS_XZ=y
CONFIG_SQUASHFS_LZ4=y
CONFIG_XFS_FS=y
CONFIG_NTFS_FS=y
CONFIG_EXFAT_FS=y
CONFIG_FUSE_FS=y
CONFIG_AUTOFS4_FS=y
CONFIG_CIFS=y
CONFIG_NFS_FS=y
CONFIG_NFS_V4=y
CONFIG_FS_VERITY=y
CONFIG_QUOTA=y
CONFIG_DM_THIN_PROVISIONING=y
CONFIG_DM_SNAPSHOT=y
CONFIG_PARTITION_ADVANCED=y

# === RAID & Block ===
CONFIG_MD=y
CONFIG_BLK_DEV_MD=y
CONFIG_MD_LINEAR=y
CONFIG_MD_RAID0=y
CONFIG_MD_RAID1=y
CONFIG_MD_RAID5=y
CONFIG_MD_RAID6=y
CONFIG_MD_RAID10=y
CONFIG_BLK_DEV_NVME=y
CONFIG_BLK_CGROUP=y
CONFIG_IOSCHED_BFQ=y

# === Virtualization / Containers ===
CONFIG_KVM=y
CONFIG_KVM_INTEL=y
CONFIG_KVM_AMD=y
CONFIG_VIRTUALIZATION=y
CONFIG_VIRTIO=y
CONFIG_VIRTIO_PCI=y
CONFIG_VIRTIO_BALLOON=y
CONFIG_VIRTIO_NET=y
CONFIG_VIRTIO_BLK=y
CONFIG_VIRTIO_CONSOLE=y
CONFIG_VHOST_NET=y
CONFIG_VHOST_VDPA=y
CONFIG_NAMESPACES=y
CONFIG_CGROUPS=y

# === Security / Encryption ===
CONFIG_FS_ENCRYPTION=y
CONFIG_KEYS=y
CONFIG_SECURITY=y
CONFIG_SECURITY_APPARMOR=y
CONFIG_LOCKDOWN_LSM=y
CONFIG_CRYPTO=y
CONFIG_IPSEC=y
CONFIG_KEYS_COMPAT=y
CONFIG_SECURITY_TOMOYO=y

# === Networking ===
CONFIG_NETFILTER=y
CONFIG_BPF=y
CONFIG_XDP_SOCKETS=y
CONFIG_TCP_CONG_BBR=y
CONFIG_BONDING=y
CONFIG_BRIDGE=y
CONFIG_VXLAN=y
CONFIG_MACVLAN=y
CONFIG_WIREGUARD=y
CONFIG_NETFILTER_XT_MATCH_IPSET=y
CONFIG_NETFILTER_XT_MATCH_CONNTRACK=y

# === GPU / Graphics ===
CONFIG_DRM=y
CONFIG_DRM_KMS_HELPER=y
CONFIG_FB=y
CONFIG_VGA_ARB=y

# === Hardware / Peripherals ===
CONFIG_USB_SUPPORT=y
CONFIG_USB_XHCI_HCD=y
CONFIG_SND=y
CONFIG_SND_HDA_INTEL=y
CONFIG_INPUT_MOUSEDEV=y
CONFIG_INPUT_KEYBOARD=y
CONFIG_BT=y
CONFIG_I2C=y
CONFIG_SPI=y

# === Modules ===
CONFIG_MODULES=y
CONFIG_MODULE_UNLOAD=y
CONFIG_MODULE_FORCE_LOAD=y

# === Compression / Archiving ===
CONFIG_ZLIB_DEFLATE=y
CONFIG_LZO_COMPRESS=y
CONFIG_XZ_DEC=y

# === Debug / Performance Tools ===
CONFIG_PERF_EVENTS=y
CONFIG_PERF_EVENTS_INTEL_UNCORE=y
CONFIG_FTRACE=y
CONFIG_FTRACE_SYSCALLS=y
CONFIG_KPROBES=y
CONFIG_EBPF_SYSCALL=y
CONFIG_DEBUG_FS=y

# === Misc / Power / ACPI ===
CONFIG_MTRR=y
CONFIG_ACPI=y
CONFIG_PM=y
CONFIG_SUSPEND=y
CONFIG_FW_LOADER=y

# === Input / Game Controllers ===
CONFIG_INPUT_JOYSTICK=y
CONFIG_HID=y
CONFIG_HID_GENERIC=y
CONFIG_HID_XPAD=y
CONFIG_HID_LOGITECH=y
CONFIG_HID_MICROSOFT=y

# === Wi-Fi / Networking ===
CONFIG_IWLWIFI=y
CONFIG_BRCMFMAC=y
CONFIG_B43=y
CONFIG_ATH9K=y
CONFIG_USB_NET_DRIVERS=y

# === Thunderbolt / USB 4 ===
CONFIG_THUNDERBOLT=y
CONFIG_USB4=y
CONFIG_INTEL_TBT=y

# === Video / PipeWire support ===
CONFIG_VIDEO_V4L2=y
CONFIG_VIDEO_VIDEOBUF2_CORE=y
CONFIG_VIDEO_VIDEOBUF2_MEMOPS=y
CONFIG_VIDEO_VIDEOBUF2_V4L2=y

# === Android Support ===
CONFIG_ANDROID_BINDER_IPC=y
CONFIG_ANDROID_BINDERFS=y
CONFIG_ANDROID_BINDER_DEVICES="binder,hwbinder,vndbinder"

# === Miscellaneous ===
CONFIG_NTSYNC=y
`

// BuildNvidiaDriver builds the NVIDIA open-gpu-kernel-modules against the
// freshly built kernel and installs them next to the kernel's own modules.
// These modules only load when matching NVIDIA hardware is present, so building
// them is harmless on non-NVIDIA machines.
func BuildNvidiaDriver(driverPath, version, kernelVersion, runixosVersion, src string) error {
	fmt.Println("Building NVIDIA driver...")
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("resolve src: %w", err)
	}

	fmt.Printf("Extracting %s to %s\n", driverPath, absSrc)
	if err := utils.ExtractTar(driverPath, absSrc); err != nil {
		return fmt.Errorf("extract NVIDIA driver: %w", err)
	}

	extractedDir := filepath.Join(absSrc, "open-gpu-kernel-modules-"+version)
	kernelSrc := filepath.Join(absSrc, "linux-"+kernelVersion)

	// TODO: drop once https://github.com/NVIDIA/open-gpu-kernel-modules/pull/940 merges
	makefilePath := filepath.Join(extractedDir, "kernel-open/Makefile")
	if err := utils.Run("", "sed", []string{"-i", "s/MODLIB :=/MODLIB ?=/g", makefilePath}); err != nil {
		return fmt.Errorf("patch NVIDIA Makefile: %w", err)
	}

	release, err := kernelRelease(kernelSrc)
	if err != nil {
		return err
	}
	dst := destDir(absSrc)
	modlib := fmt.Sprintf("%s/Core/LibKit/modules/%s", dst, release)
	common := []string{
		"KERNEL_SOURCE=" + kernelSrc,
		"KERNEL_MODLIB=" + modlib,
		"KERNEL_UNAME=" + release,
	}

	if err := utils.Make(extractedDir, "modules", common...); err != nil {
		return err
	}
	if err := utils.Make(extractedDir, "modules_install",
		append(common, "INSTALL_MOD_PATH="+dst+"/Core/LibKit")...); err != nil {
		return err
	}
	return nil
}
