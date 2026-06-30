package collector

import (
	"context"
	"errors"
	"testing"

	"github.com/Vrex123/pharos/internal/model"
)

// fakeRunner returns canned responses per remote command.
type fakeRunner struct {
	responses map[string]string
	errs      map[string]error
}

func (f *fakeRunner) Run(_ context.Context, _ model.Server, command string) (string, error) {
	if err, ok := f.errs[command]; ok {
		return "", err
	}
	return f.responses[command], nil
}

func (f *fakeRunner) InteractiveSSH(model.Server) error                    { return nil }
func (f *fakeRunner) InteractiveContainerShell(model.Server, string) error { return nil }
func (f *fakeRunner) InteractiveContainerLogs(model.Server, string) error  { return nil }

func dockerServer() model.Server {
	return model.Server{Name: "prod", Host: "h", Port: 22, User: "root", Docker: true}
}

func TestCollectServerOnline(t *testing.T) {
	r := &fakeRunner{responses: map[string]string{
		"echo ok":          "ok\n",
		StatsCommand:       "LOAD=0.10 0.20 0.30 1/1 1\nMem: 100 40 60\nDISK=1000 400 600 40% /\n",
		ProcessesCommand:   topSample,
		DockerPSCommand:    `{"ID":"a","Image":"img","Names":"web","State":"running","Status":"Up"}` + "\n",
		DockerStatsCommand: `{"Name":"web","CPUPerc":"1%","MemUsage":"5MiB / 1GiB"}` + "\n",
	}}

	snap := CollectServer(context.Background(), r, dockerServer())
	if !snap.Online {
		t.Fatalf("expected online, error=%q", snap.Error)
	}
	if snap.Error != "" {
		t.Errorf("unexpected error: %q", snap.Error)
	}
	if snap.Stats.Load1 != 0.10 {
		t.Errorf("load1 = %v", snap.Stats.Load1)
	}
	if len(snap.Containers) != 1 || snap.Containers[0].Name != "web" {
		t.Errorf("containers = %+v", snap.Containers)
	}
	if _, ok := snap.DockerStats["web"]; !ok {
		t.Errorf("missing web stats: %v", snap.DockerStats)
	}
	if len(snap.Processes) != 2 {
		t.Errorf("expected 2 processes, got %+v", snap.Processes)
	}
}

func TestCollectServerOffline(t *testing.T) {
	r := &fakeRunner{errs: map[string]error{"echo ok": errors.New("connection refused")}}
	snap := CollectServer(context.Background(), r, dockerServer())
	if snap.Online {
		t.Fatal("expected offline")
	}
	if snap.Error == "" {
		t.Error("expected error message")
	}
}

func TestCollectServerDockerErrorStaysOnline(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]string{
			"echo ok":        "ok\n",
			StatsCommand:     "LOAD=0.1 0.2 0.3\nMem: 1 1 1\nDISK=1 1 1 1% /\n",
			ProcessesCommand: topSample,
		},
		errs: map[string]error{
			DockerPSCommand:    errors.New("permission denied while trying to connect to the Docker daemon socket"),
			DockerStatsCommand: errors.New("permission denied"),
		},
	}
	snap := CollectServer(context.Background(), r, dockerServer())
	if !snap.Online {
		t.Fatal("docker error should not mark server offline")
	}
	if snap.Error == "" {
		t.Error("expected docker error recorded")
	}
}

func TestCollectServerNoDocker(t *testing.T) {
	r := &fakeRunner{responses: map[string]string{
		"echo ok":        "ok\n",
		StatsCommand:     "LOAD=0.1 0.2 0.3\nMem: 1 1 1\nDISK=1 1 1 1% /\n",
		ProcessesCommand: topSample,
	}}
	s := dockerServer()
	s.Docker = false
	snap := CollectServer(context.Background(), r, s)
	if !snap.Online || snap.Error != "" {
		t.Errorf("online=%v err=%q", snap.Online, snap.Error)
	}
	if len(snap.Containers) != 0 {
		t.Errorf("expected no containers, got %+v", snap.Containers)
	}
	if len(snap.Processes) != 2 {
		t.Errorf("expected processes on non-docker host, got %+v", snap.Processes)
	}
}
