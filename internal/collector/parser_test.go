package collector

import "testing"

func TestParseStats(t *testing.T) {
	raw := "LOAD=0.15 0.10 0.05 1/234 5678\n" +
		"CORES=8\n" +
		"MEM=              total        used        free      shared  buff/cache   available\n" +
		"Mem:     4096000000  1400000000  2000000000      100000   696000000  2500000000\n" +
		"Swap:             0           0           0\n" +
		"DISK=85899345920 45097156608 36507942912 56% /\n"

	stats, err := ParseStats(raw)
	if err != nil {
		t.Fatalf("ParseStats: %v", err)
	}
	if stats.Load1 != 0.15 || stats.Load5 != 0.10 || stats.Load15 != 0.05 {
		t.Errorf("load = %v/%v/%v", stats.Load1, stats.Load5, stats.Load15)
	}
	if stats.CPUCores != 8 {
		t.Errorf("cores = %d, want 8", stats.CPUCores)
	}
	if stats.MemTotal != 4096000000 || stats.MemUsed != 1400000000 || stats.MemFree != 2000000000 {
		t.Errorf("mem = %d/%d/%d", stats.MemTotal, stats.MemUsed, stats.MemFree)
	}
	if stats.DiskTotal != 85899345920 || stats.DiskUsed != 45097156608 || stats.DiskAvail != 36507942912 {
		t.Errorf("disk = %d/%d/%d", stats.DiskTotal, stats.DiskUsed, stats.DiskAvail)
	}
	if stats.DiskPercent != "56%" {
		t.Errorf("disk pct = %q", stats.DiskPercent)
	}
}

func TestParseStatsEmpty(t *testing.T) {
	if _, err := ParseStats("garbage\n"); err == nil {
		t.Fatal("expected error for unrecognizable output")
	}
}

func TestParseStatsNoCores(t *testing.T) {
	// Output from a host without nproc still parses; cores defaults to 0.
	stats, err := ParseStats("LOAD=0.15 0.10 0.05 1/234 5678\n")
	if err != nil {
		t.Fatalf("ParseStats: %v", err)
	}
	if stats.CPUCores != 0 {
		t.Errorf("cores = %d, want 0", stats.CPUCores)
	}
}

func TestParseDockerPS(t *testing.T) {
	raw := `{"ID":"abc123","Image":"nginx:latest","Names":"web","State":"running","Status":"Up 2 hours"}
{"ID":"def456","Image":"postgres:16","Names":"db","State":"running","Status":"Up 2 hours"}
`
	cs, err := ParseDockerPS(raw)
	if err != nil {
		t.Fatalf("ParseDockerPS: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("got %d containers, want 2", len(cs))
	}
	if cs[0].Name != "web" || cs[0].Image != "nginx:latest" || cs[0].State != "running" {
		t.Errorf("container[0] = %+v", cs[0])
	}
	if cs[1].ID != "def456" {
		t.Errorf("container[1] id = %q", cs[1].ID)
	}
}

func TestParseDockerPSSkipsMalformed(t *testing.T) {
	raw := "{\"ID\":\"a\",\"Names\":\"ok\"}\nnot json\n{bad}\n"
	cs, err := ParseDockerPS(raw)
	if err != nil {
		t.Fatalf("ParseDockerPS: %v", err)
	}
	if len(cs) != 1 || cs[0].Name != "ok" {
		t.Errorf("expected 1 valid container, got %+v", cs)
	}
}

func TestParseDockerStats(t *testing.T) {
	raw := `{"Container":"abc123","Name":"web","CPUPerc":"1.25%","MemUsage":"50MiB / 1GiB","MemPerc":"4.88%","NetIO":"1kB / 2kB","BlockIO":"0B / 0B","PIDs":"12"}
`
	m, err := ParseDockerStats(raw)
	if err != nil {
		t.Fatalf("ParseDockerStats: %v", err)
	}
	cs, ok := m["web"]
	if !ok {
		t.Fatalf("missing 'web' key in %v", m)
	}
	if cs.CPUPerc != "1.25%" || cs.MemUsage != "50MiB / 1GiB" || cs.PIDs != "12" {
		t.Errorf("stats = %+v", cs)
	}
	if cs.ID != "abc123" {
		t.Errorf("id = %q, want fallback to Container", cs.ID)
	}
}

// topSample is two iterations of `top -bn2` output. The first iteration lists
// systemd; the second (final) lists dockerd and postgres. ParseProcesses should
// read only the second block.
const topSample = `top - 15:04:05 up 1 day,  2:33,  1 user,  load average: 0.18, 0.10, 0.05
Tasks: 120 total,   1 running, 119 sleeping,   0 stopped,   0 zombie
%Cpu(s):  2.0 us,  1.0 sy,  0.0 ni, 97.0 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
MiB Mem :   3939.0 total,    500.0 free,   1200.0 used,   2239.0 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.   2400.0 avail Mem

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
      1 root      20   0  100000  10000   5000 S   0.0   0.2   0:01.23 systemd

top - 15:04:06 up 1 day,  2:33,  1 user,  load average: 0.18, 0.10, 0.05
Tasks: 120 total,   2 running, 118 sleeping,   0 stopped,   0 zombie
%Cpu(s):  5.0 us,  2.0 sy,  0.0 ni, 93.0 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
MiB Mem :   3939.0 total,    500.0 free,   1200.0 used,   2239.0 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.   2400.0 avail Mem

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
   1234 root      20   0  500000  50000  10000 S  12.3   4.1   1:02.34 dockerd
    567 postgres  20   0  200000  80000  20000 S   3.2   8.0   0:45.10 postgres: writer process
`

func TestParseProcesses(t *testing.T) {
	procs, err := ParseProcesses(topSample)
	if err != nil {
		t.Fatalf("ParseProcesses: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("expected 2 processes from the final iteration, got %d: %+v", len(procs), procs)
	}
	for _, p := range procs {
		if p.PID == "1" || p.Command == "systemd" {
			t.Errorf("first iteration leaked into output: %+v", p)
		}
	}
	p0 := procs[0]
	if p0.PID != "1234" || p0.User != "root" || p0.CPUPerc != "12.3" ||
		p0.MemPerc != "4.1" || p0.Time != "1:02.34" || p0.Command != "dockerd" {
		t.Errorf("proc[0] = %+v", p0)
	}
	if procs[1].Command != "postgres: writer process" {
		t.Errorf("multi-word command not joined: %q", procs[1].Command)
	}
}

func TestParseProcessesNoHeader(t *testing.T) {
	if _, err := ParseProcesses("garbage\nmore garbage\n"); err == nil {
		t.Fatal("expected error when no top header is present")
	}
}
