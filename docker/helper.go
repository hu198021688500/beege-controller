package docker

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Cpu struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	Irq     uint64
	SoftIrq uint64
}

func GetCpu() (*Cpu, error) {
	cpu := &Cpu{}
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return cpu, err
	}
	reader := bufio.NewReader(bytes.NewBuffer(contents))
	for {
		data, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		line := string(data)
		if line[0:4] != "cpu " {
			continue
		}
		fields := strings.Fields(line)
		if cpu.User, err = strconv.ParseUint(fields[1], 10, 64); err != nil {
			return cpu, err
		}
		if cpu.Nice, err = strconv.ParseUint(fields[2], 10, 64); err != nil {
			return cpu, err
		}
		if cpu.System, err = strconv.ParseUint(fields[3], 10, 64); err != nil {
			return cpu, err
		}
		if cpu.Idle, err = strconv.ParseUint(fields[4], 10, 64); err != nil {
			return cpu, err
		}
		if cpu.IOWait, err = strconv.ParseUint(fields[5], 10, 64); err != nil {
			return cpu, err
		}
		if cpu.Irq, err = strconv.ParseUint(fields[6], 10, 64); err != nil {
			return cpu, err
		}
		if cpu.SoftIrq, err = strconv.ParseUint(fields[7], 10, 64); err != nil {
			return cpu, err
		}
		return cpu, nil
	}
	return cpu, nil
}

func GetIdleAndTotal() (idle, total uint64) {
	cpu, _ := GetCpu()
	return cpu.Idle, cpu.User + cpu.Nice + cpu.System +
		cpu.Idle + cpu.IOWait + cpu.Irq + cpu.SoftIrq
}

func GetCpuUsage() float64 {
	idle0, total0 := GetIdleAndTotal()
	time.Sleep(3 * time.Second)
	idle1, total1 := GetIdleAndTotal()
	idleTicks := float64(idle1 - idle0)
	totalTicks := float64(total1 - total0)
	return 100 * (totalTicks - idleTicks) / totalTicks
}

type LoadAverage struct {
	One, Five, Fifteen float64
}

func GetLoadAverage() (*LoadAverage, error) {
	loadAverage := &LoadAverage{}
	line, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return loadAverage, err
	}
	fields := strings.Fields(string(line))
	loadAverage.One, err = strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return loadAverage, err
	}
	loadAverage.Five, err = strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return loadAverage, err
	}
	loadAverage.Fifteen, err = strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return loadAverage, err
	}
	return loadAverage, nil
}

type Mem struct {
	Total      uint64
	Used       uint64
	Free       uint64
	ActualFree uint64
	ActualUsed uint64
}

func GetMem() (*Mem, error) {
	var mem *Mem = &Mem{}
	var buffers, cached uint64
	table := map[string]*uint64{
		"MemTotal": &mem.Total,
		"MemFree":  &mem.Free,
		"Buffers":  &buffers,
		"Cached":   &cached,
	}
	contents, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return mem, err
	}
	reader := bufio.NewReader(bytes.NewBuffer(contents))
	for {
		data, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		line := string(data)
		fields := strings.Split(line, ":")
		if ptr := table[fields[0]]; ptr != nil {
			num := strings.TrimLeft(fields[1], " ")
			val, err := strconv.ParseUint(strings.Fields(num)[0], 10, 64)
			if err == nil {
				*ptr = val * 1024
			}
		}
	}
	mem.Used = mem.Total - mem.Free
	kern := buffers + cached
	mem.ActualFree = mem.Free + kern
	mem.ActualUsed = mem.Used - kern
	return mem, nil
}

type Swap struct {
	Total uint64
	Used  uint64
	Free  uint64
}

func GetSwap() (*Swap, error) {
	swap := &Swap{}
	sysinfo := syscall.Sysinfo_t{}
	if err := syscall.Sysinfo(&sysinfo); err != nil {
		return swap, err
	}
	swap.Total = sysinfo.Totalswap
	swap.Free = sysinfo.Freeswap
	swap.Used = swap.Total - swap.Free
	return swap, nil
}
