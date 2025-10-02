package builder

import "fmt"
import "os"
import "path/filepath"
import "KernelFactory/utils"
import "os/exec"

func BuildKernel(out *os.File, version, src string) {
	fmt.Println("Building kernel...")
	//return full path of current working directory, not ".", but /home/ren/coding/KernelFactory
	pwd := filepath.Dir(out.Name())
	pwd, _ = filepath.Abs(pwd)
	fmt.Printf("Current working directory: %s\n", pwd)
	// Extract the tar.xz file
	fmt.Printf("Extracting %s to %s\n", out.Name(), src)

	err := utils.ExtractTar(out.Name(), src)
	if err != nil {
		fmt.Println("Error extracting kernel:", err)
		return
	}

	fmt.Println("Kernel extracted successfully")
	// Set local version
	err = SetLocalVersion(fmt.Sprintf("%s/linux-%s", src, version), "-rovelos-1")
	if err != nil {
		fmt.Println("Error setting local version:", err)
		return
	}

	fmt.Println("Local version set successfully")

	/* START BORE PATCH */
	pathURL := "https://raw.githubusercontent.com/firelzrd/bore-scheduler/refs/heads/main/patches/stable/linux-6.17-bore/0001-linux6.17-rc4-bore-6.5.5.patch"
	patchPath := fmt.Sprintf("%s/linux-%s/bore.patch", src, version)
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		cmd := exec.Command("wget", "-O", patchPath, pathURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			fmt.Println("Error downloading bore patch:", err)
			return
		}
	}
	cmd := exec.Command("patch", "-p1", "-i", "./bore.patch")
	cmd.Dir = fmt.Sprintf("%s/linux-%s", src, version)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error applying bore patch:", err)
		return
	}
	fmt.Println("Bore patch applied successfully")
	/* END BORE PATCH */

	utils.MakeWithAllCores(fmt.Sprintf("%s/linux-%s", src, version), "defconfig")
	cmd = exec.Command("bash", "-lc", "scripts/kconfig/merge_config.sh -m .config THECONFIG")
	cmd.Dir = fmt.Sprintf("%s/linux-%s", src, version)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error merging config:", err)
		return
	}
	utils.MakeWithAllCores(fmt.Sprintf("%s/linux-%s", src, version), "olddefconfig")
	utils.MakeWithAllCores(fmt.Sprintf("%s/linux-%s", src, version), "bzImage modules")
	utils.MakeWithAllCores(fmt.Sprintf("%s/linux-%s", src, version), "install", fmt.Sprintf("INSTALL_PATH=%s/%s/boot", pwd, src))
	utils.MakeWithAllCores(fmt.Sprintf("%s/linux-%s", src, version), "modules_install", fmt.Sprintf("INSTALL_MOD_PATH=%s/%s/modules", pwd, src))
}

func SetLocalVersion(src, ver string) error {
	f, err := os.Create(fmt.Sprintf("%s/localversion", src))
	if err != nil {
		return fmt.Errorf("failed to create localversion file: %v", err)
	}
	defer f.Close()
	_, err = f.WriteString(ver)
	if err != nil {
		return fmt.Errorf("failed to write to localversion file: %v", err)
	}
	f, err = os.Create(fmt.Sprintf("%s/THECONFIG", src))
	if err != nil {
		return fmt.Errorf("failed to create THECONFIG file: %v", err)
	}
	defer f.Close()
	_, err = f.WriteString(`# === Timer & Preemption (Hybrid Tickless - Windows like) ===
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
`)
	if err != nil {
		return fmt.Errorf("failed to write to THECONFIG file: %v", err)
	}
	return nil
}

func BuildNvidiaDriver(out *os.File, version, src string) {
	fmt.Println("Building NVIDIA driver...")
	// Extract the tar.gz file
	fmt.Printf("Extracting %s to %s\n", out.Name(), src)
	pwd := filepath.Dir(out.Name())
	pwd, _ = filepath.Abs(pwd)
	fmt.Printf("Current working directory: %s\n", pwd)

	err := utils.ExtractTar(out.Name(), src)
	if err != nil {
		fmt.Println("Error extracting NVIDIA driver:", err)
		return
	}

	fmt.Println("NVIDIA driver extracted successfully")
	// Change directory to the extracted folder
	extractedDir := fmt.Sprintf("%s/open-gpu-kernel-modules-%s", src, version)
	// Run make with all cores

	//TODO: remove this if https://github.com/NVIDIA/open-gpu-kernel-modules/pull/940 is merged
	//replace "MODLIB :=" with "MODLIB ?=" in extractedDir/kernel-open/Makefile file, using sed -i 's/MODLIB :=/MODLIB ?=/g' filename
	makefilePath := fmt.Sprintf("%s/kernel-open/Makefile", extractedDir)
	cmd := exec.Command("sed", "-i", "s/MODLIB :=/MODLIB ?=/g", makefilePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error modifying Makefile:", err)
		return
	}

	utils.MakeWithAllCores(extractedDir, "modules", fmt.Sprintf("KERNEL_SOURCE=%s/%s/linux-%s", pwd, src, version), fmt.Sprintf("KERNEL_MODLIB=%s/output/modules/lib/modules/6.17.0-rovelos-1", pwd), fmt.Sprintf("KERNEL_UNAME=%s", "6.17.0-rovelos-1"))
	utils.MakeWithAllCores(extractedDir, "modules_install", fmt.Sprintf("KERNEL_SOURCE=%s/%s/linux-%s", pwd, src, version), fmt.Sprintf("KERNEL_MODLIB=%s/output/modules/lib/modules/6.17.0-rovelos-1", pwd), fmt.Sprintf("KERNEL_UNAME=%s", "6.17.0-rovelos-1"),
		fmt.Sprintf("INSTALL_MOD_PATH=%s/output/modules", pwd))
}
