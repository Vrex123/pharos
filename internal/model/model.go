// Package model holds the core data types shared across pharos: server
// definitions and the snapshots collected from them.
package model

import "time"

// Server is a single SSH target from the config file.
type Server struct {
	Name         string
	Host         string
	Port         int
	User         string
	IdentityFile string
	Docker       bool
}

// ServerStats holds the load/memory/disk metrics scraped from a server.
// For the MVP "CPU" is reported as load average, not a true CPU percentage.
type ServerStats struct {
	Load1       float64
	Load5       float64
	Load15      float64
	CPUCores    int
	MemTotal    uint64
	MemUsed     uint64
	MemFree     uint64
	DiskTotal   uint64
	DiskUsed    uint64
	DiskAvail   uint64
	DiskPercent string
}

// Container is a Docker container parsed from `docker ps`.
type Container struct {
	ID     string
	Name   string
	Image  string
	State  string
	Status string
}

// ContainerStats is a single row from `docker stats --no-stream`.
type ContainerStats struct {
	ID       string
	Name     string
	CPUPerc  string
	MemUsage string
	MemPerc  string
	NetIO    string
	BlockIO  string
	PIDs     string
}

// Process is a single OS process scraped from `top`. Fields are kept as display
// strings, mirroring ContainerStats, since top's output is display-oriented.
type Process struct {
	PID     string
	User    string
	CPUPerc string // %CPU
	MemPerc string // %MEM
	Time    string // TIME+
	Command string
}

// ServerSnapshot is the aggregated result of collecting from one server.
// DockerStats is keyed by container name (falling back to ID).
type ServerSnapshot struct {
	Server      Server
	Online      bool
	Error       string
	Stats       ServerStats
	Containers  []Container
	DockerStats map[string]ContainerStats
	Processes   []Process
	UpdatedAt   time.Time
}
