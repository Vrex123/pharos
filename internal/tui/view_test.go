package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/Vrex123/pharos/internal/model"
)

func testModel() AppModel {
	servers := []model.Server{
		{Name: "prod", Host: "1.2.3.4", Port: 22, User: "root", Docker: true},
		{Name: "staging", Host: "staging.example.com", Port: 22, User: "deploy", Docker: true},
	}
	m := newModel(servers, nil, "")
	m.width, m.height = 110, 40
	return m
}

func TestViewLoadingBeforeSize(t *testing.T) {
	m := testModel()
	m.width = 0
	if !strings.Contains(m.View(), "Loading") {
		t.Errorf("expected loading placeholder, got %q", m.View())
	}
}

func TestViewRendersPanels(t *testing.T) {
	m := testModel()
	out := m.View()
	for _, want := range []string{"Servers", "Server Stats", "Docker Containers", "prod", "staging", "tab switch focus"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q\n---\n%s", want, out)
		}
	}
}

func TestViewFooterHintsFollowFocus(t *testing.T) {
	m := testModel() // default focus is Servers

	out := m.View()
	for _, want := range []string{"add", "ssh"} {
		if !strings.Contains(out, want) {
			t.Errorf("servers-focus footer missing %q\n---\n%s", want, out)
		}
	}
	for _, notWant := range []string{"exec", "logs"} {
		if strings.Contains(out, notWant) {
			t.Errorf("servers-focus footer should not contain %q\n---\n%s", notWant, out)
		}
	}

	m.focus = FocusContainers
	out = m.View()
	for _, want := range []string{"exec", "logs"} {
		if !strings.Contains(out, want) {
			t.Errorf("containers-focus footer missing %q\n---\n%s", want, out)
		}
	}
	for _, notWant := range []string{"add", "ssh"} {
		if strings.Contains(out, notWant) {
			t.Errorf("containers-focus footer should not contain %q\n---\n%s", notWant, out)
		}
	}
}

func TestFooterHintsWrapToWidth(t *testing.T) {
	const width = 40
	out := footerHints(FocusServers, width)
	if !strings.Contains(out, "\n") {
		t.Fatalf("expected hints to wrap at width %d, got single line:\n%s", width, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if w := len([]rune(line)); w > width {
			t.Errorf("line exceeds width %d (%d): %q", width, w, line)
		}
	}

	// width <= 0 disables wrapping (single line).
	if strings.Contains(footerHints(FocusServers, 0), "\n") {
		t.Errorf("width 0 should not wrap")
	}
}

func TestViewOnlineServerStats(t *testing.T) {
	m := testModel()
	m.snapshots["prod"] = model.ServerSnapshot{
		Server:    m.servers[0],
		Online:    true,
		UpdatedAt: time.Now(),
		Stats: model.ServerStats{
			Load1: 0.18, Load5: 0.10, Load15: 0.05, CPUCores: 4,
			MemTotal: 4 << 30, MemUsed: 1 << 30,
			DiskTotal: 80 << 30, DiskUsed: 42 << 30, DiskPercent: "52%",
		},
		Containers: []model.Container{
			{ID: "abc", Name: "web", Image: "app:latest", State: "running", Status: "Up"},
		},
		DockerStats: map[string]model.ContainerStats{
			"web": {Name: "web", CPUPerc: "2.1%", MemUsage: "180MiB / 1GiB"},
		},
	}
	out := m.View()
	for _, want := range []string{"online", "load avg:", "0.18 (5%)", "0.10 (3%)", "0.05 (1%)", "4 cores", "ram:", "25%", "disk:", "52%", "web", "app:latest", "2.1%"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q\n---\n%s", want, out)
		}
	}
}

func TestViewOfflineServer(t *testing.T) {
	m := testModel()
	m.snapshots["prod"] = model.ServerSnapshot{
		Server: m.servers[0],
		Online: false,
		Error:  "ssh prod: connection refused",
	}
	out := m.View()
	if !strings.Contains(out, "offline") {
		t.Errorf("expected offline status\n%s", out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("expected error surfaced\n%s", out)
	}
}

func TestMoveSelectionAndFocus(t *testing.T) {
	m := testModel()
	if m.selectedServer != 0 {
		t.Fatal("expected initial server 0")
	}
	m.moveSelection(1)
	if m.selectedServer != 1 {
		t.Errorf("selectedServer = %d, want 1", m.selectedServer)
	}
	m.moveSelection(5) // clamp
	if m.selectedServer != 1 {
		t.Errorf("selectedServer = %d, want clamped 1", m.selectedServer)
	}
	m.moveSelection(-10) // clamp low
	if m.selectedServer != 0 {
		t.Errorf("selectedServer = %d, want clamped 0", m.selectedServer)
	}
}
