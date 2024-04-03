package worker

import (
	"fmt"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"time"
)

type Stats struct {
	MemStats  *mem.VirtualMemoryStat
	DiskStats *disk.UsageStat
	CpuStats  *cpu.InfoStat
	LoadStats *load.AvgStat
	TaskCount int
}

func (s *Stats) MemTotalKb() uint64 {
	return s.MemStats.Total
}

func (s *Stats) MemAvailableKb() uint64 {
	return s.MemStats.Available
}

func (s *Stats) MemUsedKb() uint64 {
	return s.MemStats.Used
}

func (s *Stats) MemUsedPercent() uint64 {
	return uint64(s.MemStats.UsedPercent)
}

func (s *Stats) DiskTotal() uint64 {
	return s.DiskStats.Total
}

func (s *Stats) DiskFree() uint64 {
	return s.DiskStats.Free
}

func (s *Stats) DiskUsed() uint64 {
	return s.DiskStats.Used
}

func (s *Stats) CpuUsage() float64 {
	interval := 0 * time.Second

	cpuPercentages, err := cpu.Percent(interval, false)
	if err != nil {
		fmt.Printf("Error getting CPU percentage: %s\n", err)
		return 0.00
	}

	return cpuPercentages[0]
}

func GetStats() *Stats {
	memStats, _ := mem.VirtualMemory()
	diskStats, _ := disk.Usage("/")
	cpuStats, _ := cpu.Info()
	loadStats, _ := load.Avg()

	return &Stats{
		MemStats:  memStats,
		DiskStats: diskStats,
		CpuStats:  &cpuStats[0],
		LoadStats: loadStats,
	}
}
