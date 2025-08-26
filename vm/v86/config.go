//go:build js && wasm

package v86

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"syscall/js"
)

func parseFlags(args []string) (map[string]any, error) {
	var (
		// Memory options
		mem string

		// Storage options
		hda   string
		hdb   string
		fda   string
		fdb   string
		cdrom string

		// Boot options
		boot     string
		bios     string
		acpi     bool
		fastboot bool

		// Kernel boot options
		kernel string
		initrd string
		append string

		// Network options
		netdev string

		// VirtFS options
		virtfs string
	)
	f := flag.NewFlagSet("v86", flag.ContinueOnError)
	f.StringVar(&mem, "m", "512M", "Set memory size")
	f.StringVar(&mem, "mem", "512M", "Set memory size")
	f.StringVar(&hda, "hda", "", "Primary hard disk image")
	f.StringVar(&hdb, "hdb", "", "Secondary hard disk image")
	f.StringVar(&fda, "fda", "", "Floppy disk A image")
	f.StringVar(&fdb, "fdb", "", "Floppy disk B image")
	f.StringVar(&cdrom, "cdrom", "", "CD-ROM image")
	f.StringVar(&boot, "boot", "c", "Boot order (a,b,c,d,n)")
	f.StringVar(&kernel, "kernel", "", "Linux kernel image (bzImage)")
	f.StringVar(&initrd, "initrd", "", "Initial ramdisk image")
	f.StringVar(&append, "append", "", "Kernel command line")
	f.StringVar(&bios, "bios", "", "BIOS image file")
	f.BoolVar(&acpi, "acpi", false, "Enable ACPI")
	f.BoolVar(&fastboot, "fastboot", false, "Enable fast boot")
	f.StringVar(&netdev, "netdev", "user,type=ne2k,relay_url=ws://localhost:7654/.well-known/ethernet", "Network device configuration")
	f.StringVar(&virtfs, "virtfs", "", "VirtFS configuration")
	if err := f.Parse(args); err != nil {
		return nil, err
	}
	memorySize, err := parseMemorySize(mem)
	if err != nil {
		return nil, err
	}
	cmdline := "console=ttyS0" //tsc=reliable mitigations=off random.trust_cpu=on
	if append != "" {
		cmdline += " " + append
	}
	return map[string]any{
		"wasm_path":        "./v86/v86.wasm",
		"screen_container": js.Global().Get("document").Call("getElementById", "screen"),
		"memory_size":      memorySize,
		"vga_memory_size":  8 * 1024 * 1024, // 8MB
		"net_device":       parseNetdev(netdev),
		"filesystem": map[string]any{
			"handle9p": js.Global().Get("wanix").Get("virtioHandle"),
		},
		"bios": map[string]any{
			"url": "./v86/seabios.bin",
		},
		"vga_bios": map[string]any{
			"url": "./v86/vgabios.bin",
		},
		"bzimage": map[string]any{
			"url": "./linux/bzImage",
		},
		"cmdline": cmdline,
	}, nil
}

// parseMemorySize parses memory size strings like "512M", "1G", etc.
func parseMemorySize(sizeStr string) (int, error) {
	if sizeStr == "" {
		return 0, nil
	}

	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)([KMGT]?)$`)
	matches := re.FindStringSubmatch(strings.ToUpper(sizeStr))
	if matches == nil {
		return 0, fmt.Errorf("invalid memory size format: %s", sizeStr)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	unit := matches[2]
	multipliers := map[string]float64{
		"":  1024 * 1024, // Default to MB if no unit
		"B": 1,
		"K": 1024,
		"M": 1024 * 1024,
		"G": 1024 * 1024 * 1024,
		"T": 1024 * 1024 * 1024 * 1024,
	}

	multiplier, ok := multipliers[unit]
	if !ok {
		multiplier = multipliers[""]
	}

	return int(value * multiplier), nil
}

// parseBootOrder converts boot string to boot order value
func parseBootOrder(bootStr string) int {
	bootMap := map[byte]int{
		'a': 0x01, // Floppy A
		'b': 0x02, // Floppy B
		'c': 0x80, // Hard disk
		'd': 0x81, // CD-ROM
		'n': 0x82, // Network
	}

	if bootStr == "" {
		return 0x80 // Default to hard disk
	}

	firstChar := bootStr[0]
	if val, ok := bootMap[firstChar]; ok {
		return val
	}
	return 0x80 // Default to hard disk
}

// parseNetdev parses network device configuration
func parseNetdev(netdev string) map[string]any {
	if netdev == "" {
		return nil
	}

	parts := strings.Split(netdev, ",")
	if len(parts) == 0 {
		return nil
	}

	mode := parts[0]
	if mode != "user" {
		return nil
	}

	config := make(map[string]any)
	for i := 1; i < len(parts); i++ {
		kv := strings.SplitN(parts[i], "=", 2)
		if len(kv) == 2 {
			config[kv[0]] = kv[1]
		}
	}
	return config
}

// parseVirtfs parses VirtFS configuration
func parseVirtfs(virtfs string) map[string]any {
	if virtfs == "" {
		return nil
	}

	parts := strings.Split(virtfs, ",")
	if len(parts) < 2 {
		return nil
	}

	mode := parts[0]
	if mode != "proxy" {
		return nil
	}

	return map[string]any{
		"proxy_url": parts[1],
	}
}
