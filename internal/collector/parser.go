// Package collector scrapes servers via a sshcmd.Runner and parses the output
// of /proc stats and the Docker CLI into model types.
package collector

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Vrex123/pharos/internal/model"
)

// StatsCommand is the single remote command used to gather load/mem/disk.
const StatsCommand = `printf "LOAD="; cat /proc/loadavg; printf "\nCORES="; nproc; printf "MEM="; free -b; printf "\nDISK="; df -B1 / --output=size,used,avail,pcent,target | tail -n +2`

// DockerPSCommand lists containers as line-delimited JSON.
const DockerPSCommand = `docker ps --format '{{json .}}'`

// DockerStatsCommand reports a single-shot stats snapshot as JSON.
const DockerStatsCommand = `docker stats --no-stream --format '{{json .}}'`

// ProcessesCommand samples top twice so %CPU reflects the interval between the
// two iterations rather than cumulative-since-start; we parse the LAST iteration.
// LC_ALL=C forces a dot decimal separator and COLUMNS=200 keeps top from
// truncating the COMMAND column. The trailing `| cat` defeats top's TTY check.
const ProcessesCommand = `LC_ALL=C COLUMNS=200 top -bn2 -d 0.5 -o %CPU | cat`

// ParseStats parses the combined output of StatsCommand into ServerStats.
// Missing or malformed sections are left zero-valued rather than failing the
// whole parse, but a complete absence of recognizable lines is an error.
func ParseStats(raw string) (model.ServerStats, error) {
	var stats model.ServerStats
	var found bool

	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "LOAD="):
			fields := strings.Fields(strings.TrimPrefix(line, "LOAD="))
			if len(fields) >= 3 {
				stats.Load1, _ = strconv.ParseFloat(fields[0], 64)
				stats.Load5, _ = strconv.ParseFloat(fields[1], 64)
				stats.Load15, _ = strconv.ParseFloat(fields[2], 64)
				found = true
			}
		case strings.HasPrefix(line, "CORES="):
			if v := strings.TrimSpace(strings.TrimPrefix(line, "CORES=")); v != "" {
				stats.CPUCores = int(parseUint(v))
				found = true
			}
		case strings.HasPrefix(line, "Mem:"):
			fields := strings.Fields(line)
			// Mem: total used free shared buff/cache available
			if len(fields) >= 4 {
				stats.MemTotal = parseUint(fields[1])
				stats.MemUsed = parseUint(fields[2])
				stats.MemFree = parseUint(fields[3])
				found = true
			}
		case strings.HasPrefix(line, "DISK="):
			fields := strings.Fields(strings.TrimPrefix(line, "DISK="))
			// size used avail pcent target
			if len(fields) >= 4 {
				stats.DiskTotal = parseUint(fields[0])
				stats.DiskUsed = parseUint(fields[1])
				stats.DiskAvail = parseUint(fields[2])
				stats.DiskPercent = fields[3]
				found = true
			}
		}
	}

	if !found {
		return stats, fmt.Errorf("no recognizable stats in output")
	}
	return stats, nil
}

// ParseDockerPS parses line-delimited JSON from `docker ps`.
func ParseDockerPS(raw string) ([]model.Container, error) {
	var containers []model.Container
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row struct {
			ID     string `json:"ID"`
			Image  string `json:"Image"`
			Names  string `json:"Names"`
			State  string `json:"State"`
			Status string `json:"Status"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			// Skip malformed lines; Docker versions vary.
			continue
		}
		containers = append(containers, model.Container{
			ID:     row.ID,
			Name:   row.Names,
			Image:  row.Image,
			State:  row.State,
			Status: row.Status,
		})
	}
	return containers, nil
}

// ParseDockerStats parses line-delimited JSON from `docker stats --no-stream`,
// keyed by container name (falling back to ID/Container).
func ParseDockerStats(raw string) (map[string]model.ContainerStats, error) {
	out := make(map[string]model.ContainerStats)
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row struct {
			Container string `json:"Container"`
			ID        string `json:"ID"`
			Name      string `json:"Name"`
			CPUPerc   string `json:"CPUPerc"`
			MemUsage  string `json:"MemUsage"`
			MemPerc   string `json:"MemPerc"`
			NetIO     string `json:"NetIO"`
			BlockIO   string `json:"BlockIO"`
			PIDs      string `json:"PIDs"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		id := row.ID
		if id == "" {
			id = row.Container
		}
		cs := model.ContainerStats{
			ID:       id,
			Name:     row.Name,
			CPUPerc:  row.CPUPerc,
			MemUsage: row.MemUsage,
			MemPerc:  row.MemPerc,
			NetIO:    row.NetIO,
			BlockIO:  row.BlockIO,
			PIDs:     row.PIDs,
		}
		key := row.Name
		if key == "" {
			key = id
		}
		if key != "" {
			out[key] = cs
		}
	}
	return out, nil
}

// ParseProcesses parses the output of ProcessesCommand (top in batch mode) into a
// CPU-sorted slice of processes. top is sampled twice; only the final iteration is
// used so %CPU reflects the interval between samples rather than cumulative time
// since start. Column positions are resolved from the header row, which makes the
// parse tolerant of minor procps column reordering. Like the other parsers it is
// lenient: malformed rows are skipped and it only errors if no header is found.
func ParseProcesses(raw string) ([]model.Process, error) {
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}

	// Find the LAST header row (top prints one per iteration) and parse the rows
	// after it, so %CPU comes from the final sample.
	headerIdx := -1
	var cols map[string]int
	for i, line := range lines {
		idx := fieldIndex(strings.Fields(line))
		if _, hasPID := idx["PID"]; !hasPID {
			continue
		}
		if _, hasCmd := idx["COMMAND"]; !hasCmd {
			continue
		}
		headerIdx, cols = i, idx
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("no process header in top output")
	}

	pidI := cols["PID"]
	cmdI := cols["COMMAND"]
	userI, hasUser := cols["USER"]
	cpuI, hasCPU := cols["%CPU"]
	memI, hasMem := cols["%MEM"]
	timeI, hasTime := cols["TIME+"]

	var procs []model.Process
	for _, line := range lines[headerIdx+1:] {
		fields := strings.Fields(line)
		if len(fields) <= cmdI {
			continue // blank line or row too short to hold a command
		}
		get := func(i int, ok bool) string {
			if !ok || i >= len(fields) {
				return ""
			}
			return fields[i]
		}
		procs = append(procs, model.Process{
			PID:     fields[pidI],
			User:    get(userI, hasUser),
			CPUPerc: get(cpuI, hasCPU),
			MemPerc: get(memI, hasMem),
			Time:    get(timeI, hasTime),
			Command: strings.Join(fields[cmdI:], " "),
		})
	}
	return procs, nil
}

// fieldIndex maps each whitespace-separated token to its first column index.
func fieldIndex(fields []string) map[string]int {
	idx := make(map[string]int, len(fields))
	for i, f := range fields {
		if _, exists := idx[f]; !exists {
			idx[f] = i
		}
	}
	return idx
}

func parseUint(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}
