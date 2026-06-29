package collector

import (
	"context"
	"strings"
	"time"

	"github.com/Vrex123/pharos/internal/model"
	"github.com/Vrex123/pharos/internal/sshcmd"
)

const healthTimeout = 3 * time.Second

// CollectServer gathers health, metrics, OS processes, and (if enabled) Docker
// state for a single server into a ServerSnapshot. It never returns an error: failures are
// recorded in the snapshot so the UI can display them without crashing. A
// Docker failure does not mark the server offline.
func CollectServer(ctx context.Context, runner sshcmd.Runner, server model.Server) model.ServerSnapshot {
	snap := model.ServerSnapshot{
		Server:      server,
		DockerStats: map[string]model.ContainerStats{},
		UpdatedAt:   time.Now(),
	}

	// Health check.
	hctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	out, err := runner.Run(hctx, server, "echo ok")
	if err != nil || !strings.Contains(out, "ok") {
		snap.Online = false
		if err != nil {
			snap.Error = err.Error()
		} else {
			snap.Error = "health check failed"
		}
		return snap
	}
	snap.Online = true

	// Server metrics.
	if raw, err := runner.Run(ctx, server, StatsCommand); err != nil {
		snap.Error = appendErr(snap.Error, "stats: "+err.Error())
	} else if stats, perr := ParseStats(raw); perr != nil {
		snap.Error = appendErr(snap.Error, "stats: "+perr.Error())
	} else {
		snap.Stats = stats
	}

	// OS processes (applies to every host, Docker or not).
	if raw, err := runner.Run(ctx, server, ProcessesCommand); err != nil {
		snap.Error = appendErr(snap.Error, "processes: "+err.Error())
	} else if procs, perr := ParseProcesses(raw); perr != nil {
		snap.Error = appendErr(snap.Error, "processes: "+perr.Error())
	} else {
		snap.Processes = procs
	}

	if !server.Docker {
		return snap
	}

	// Docker containers.
	if raw, err := runner.Run(ctx, server, DockerPSCommand); err != nil {
		snap.Error = appendErr(snap.Error, "docker ps: "+err.Error())
	} else if containers, perr := ParseDockerPS(raw); perr != nil {
		snap.Error = appendErr(snap.Error, "docker ps: "+perr.Error())
	} else {
		snap.Containers = containers
	}

	// Docker stats.
	if raw, err := runner.Run(ctx, server, DockerStatsCommand); err != nil {
		snap.Error = appendErr(snap.Error, "docker stats: "+err.Error())
	} else if ds, perr := ParseDockerStats(raw); perr != nil {
		snap.Error = appendErr(snap.Error, "docker stats: "+perr.Error())
	} else {
		snap.DockerStats = ds
	}

	return snap
}

func appendErr(existing, msg string) string {
	if existing == "" {
		return msg
	}
	return existing + "; " + msg
}
