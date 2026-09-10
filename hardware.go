package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Inventory is evidence for conservative defaults, not a benchmark or a claim
// that every advertised accelerator supports this model.
type HardwareProfile struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Threads int    `json:"threads"`
	CPU     string `json:"cpu"`
	Memory  uint64 `json:"memory_bytes"`
	GPUs    string `json:"gpus"`
	Note    string `json:"note"`
}
type ImageRecommendation struct {
	Profile string      `json:"profile"`
	Config  ImageConfig `json:"config"`
	Width   int         `json:"width"`
	Height  int         `json:"height"`
	Steps   int         `json:"steps"`
	Reason  string      `json:"reason"`
}

var machineOnce sync.Once
var machine HardwareProfile

func hardwareProfile() HardwareProfile {
	machineOnce.Do(func() {
		machine = HardwareProfile{OS: runtime.GOOS, Arch: runtime.GOARCH, Threads: runtime.NumCPU(), Memory: physicalMemory(), Note: "Inventory only. Setup probes the actual runtime; unsupported GPUs fall back to CPU. NPU execution is not implemented."}
		switch runtime.GOOS {
		case "linux":
			b, _ := os.ReadFile("/proc/cpuinfo")
			for _, l := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(l, "model name") {
					machine.CPU = strings.TrimSpace(strings.SplitN(l, ":", 2)[1])
					break
				}
			}
			machine.GPUs = hardwareCommand("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader")
		case "windows":
			machine.CPU = os.Getenv("PROCESSOR_IDENTIFIER")
			machine.GPUs = hardwareCommand("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name")
		case "darwin":
			machine.CPU = hardwareCommand("sysctl", "-n", "machdep.cpu.brand_string")
			machine.GPUs = "Metal availability is tested during setup."
		}
		if machine.CPU == "" {
			machine.CPU = runtime.GOARCH
		}
		if machine.GPUs == "" {
			machine.GPUs = "Not reported by inventory; setup still probes available devices."
		}
	})
	return machine
}
func hardwareCommand(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b, err := nativeCommand(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return shortText(string(b), 1500)
}
func recommendImage(h HardwareProfile, profile string, accelerated bool) ImageRecommendation {
	r := ImageRecommendation{Profile: profile, Config: ImageConfig{Backend: "auto", Threads: minInt(16, maxInt(1, h.Threads-2)), MaxMinutes: 60}, Width: 512, Height: 512, Steps: 8}
	switch profile {
	case "draft":
		r.Width = 256
		r.Height = 256
		r.Steps = 4
	case "detail":
		r.Steps = 9
		if accelerated {
			r.Width = 768
			r.Height = 768
			if h.Memory >= 30<<30 {
				r.Width = 1024
				r.Height = 1024
			}
		}
	default:
		r.Profile = "balanced"
	}
	if h.Memory > 0 && h.Memory < 14<<30 {
		r.Width = 256
		r.Height = 256
		r.Config.Threads = minInt(r.Config.Threads, 4)
	}
	r.Reason = fmt.Sprintf("%s starting point: %d × %d, %d steps. Settings remain editable. Quality and elapsed time must be judged from actual results.", r.Profile, r.Width, r.Height, r.Steps)
	if h.Memory > 0 && h.Memory < 14<<30 {
		r.Reason += " Limited RAM: generation may still run out of memory; use a larger worker PC if needed."
	}
	return r
}
func (s *NativeImages) hardwareRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/images/hardware", func(w http.ResponseWriter, r *http.Request) {
		var q struct{ Profile string }
		if err := decode(r, &q); err != nil {
			apiError(w, err)
			return
		}
		h := hardwareProfile()
		s.mu.Lock()
		gpu := s.ready && (s.actualBackend == "vulkan" || s.actualBackend == "metal")
		s.mu.Unlock()
		jsonReply(w, map[string]any{"hardware": h, "recommendation": recommendImage(h, q.Profile, gpu)})
	})
}
func parseMemoryKB(text string) uint64 {
	for _, l := range strings.Split(text, "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && f[0] == "MemTotal:" {
			n, _ := strconv.ParseUint(f[1], 10, 64)
			return n * 1024
		}
	}
	return 0
}
