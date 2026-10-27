package analysis

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"

	domainanalysis "github.com/wensboy/automan/backend/domain/analysis"
)

func pidOf() int { return os.Getpid() }

// SystemInfo 采集平台信息(见 v25 §6)。
//
// 无法通过简单原生库实现的项先返回 "-": GPU / WebView 版本 / Display。
func (s *Service) SystemInfo() []domainanalysis.SystemItem {
	rows := []string{}

	var uts syscall.Utsname
	osName, kernel, arch := runtime.GOOS, "", runtime.GOARCH
	if err := syscall.Uname(&uts); err == nil {
		kernel = charsToString(uts.Release[:])
		arch = charsToString(uts.Machine[:])
		if machine := charsToString(uts.Sysname[:]); machine != "" {
			osName = machine
		}
	}

	distribution, version := osRelease()

	cpuModel := "-"
	cores := runtime.NumCPU()
	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		cpuModel = strings.TrimSpace(info[0].ModelName)
		cores = int(info[0].Cores) * len(info)
		if info[0].Cores == 0 {
			cores = runtime.NumCPU()
		}
	}
	memory := "-"
	if vm, err := mem.VirtualMemory(); err == nil {
		memory = formatGB(vm.Total)
	}
	uptime := time.Since(s.started).Round(time.Second)
	if s.started.IsZero() {
		uptime = 0
	}
	hostname, _ := os.Hostname()
	if info, err := host.Info(); err == nil && hostname == "" {
		hostname = info.Hostname
	}

	rows = append(rows,
		osName, distribution, version, kernel, arch,
		cpuModel, strconv.Itoa(cores), memory,
		"-", // GPU: 需平台特定工具, 本期不实现
		runtime.Version(), runtime.GOOS+" ("+runtime.GOARCH+")",
		"-", // WebView 版本: 需平台探测, 本期不实现
		hostname,
		os.Getenv("LANG"), time.Now().Location().String(),
		"-", // Display: 需窗口引擎提供, 本期不实现
		formatDuration(uptime),
	)

	keys := []string{
		"Operating system", "Distribution", "Version", "Kernel", "Architecture",
		"CPU", "CPU cores", "Memory",
		"GPU",
		"Runtime", "Platform",
		"WebView",
		"Hostname",
		"Locale", "Timezone",
		"Display",
		"Uptime",
	}

	items := make([]domainanalysis.SystemItem, 0, len(keys))
	for index, key := range keys {
		value := "-"
		if index < len(rows) && rows[index] != "" {
			value = rows[index]
		}
		items = append(items, domainanalysis.SystemItem{ID: int64(index + 1), Key: key, Value: value})
	}
	return items
}

func charsToString[T ~int8 | ~uint8](chars []T) string {
	buf := make([]byte, 0, len(chars))
	for _, char := range chars {
		if char == 0 {
			break
		}
		buf = append(buf, byte(char))
	}
	return string(buf)
}

func osRelease() (string, string) {
	payload, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "-", "-"
	}
	distribution, version := "-", "-"
	for _, line := range strings.Split(string(payload), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "NAME=") {
			distribution = strings.Trim(strings.TrimPrefix(line, "NAME="), `"`)
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
		}
	}
	return distribution, version
}

func formatGB(bytes uint64) string {
	const gb = 1024 * 1024 * 1024
	if bytes == 0 {
		return "-"
	}
	return strconv.FormatFloat(float64(bytes)/float64(gb), 'f', 1, 64) + " GB"
}

func formatDuration(value time.Duration) string {
	if value <= 0 {
		return "-"
	}
	days := int(value.Hours()) / 24
	hours := int(value.Hours()) % 24
	minutes := int(value.Minutes()) % 60
	seconds := int(value.Seconds()) % 60
	return strings.TrimSpace(strings.Join([]string{
		strconv.Itoa(days) + "d",
		pad2(hours) + ":" + pad2(minutes) + ":" + pad2(seconds),
	}, " "))
}

func pad2(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
