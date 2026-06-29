package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Vrex123/pharos/internal/config"
	"github.com/Vrex123/pharos/internal/model"
	tea "github.com/charmbracelet/bubbletea"
)

// sendKey feeds a key string through Update and returns the updated AppModel.
func sendKey(t *testing.T, m AppModel, s string) AppModel {
	t.Helper()
	var msg tea.KeyMsg
	switch s {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	next, _ := m.Update(msg)
	return next.(AppModel)
}

func TestEmptyStateView(t *testing.T) {
	m := newModel(nil, nil, "")
	m.width, m.height = 110, 40
	out := m.View()
	if !strings.Contains(out, "no servers") || !strings.Contains(out, "press 'a'") {
		t.Errorf("empty-state hint missing:\n%s", out)
	}
}

func TestAddServerFlowPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	m := newModel(nil, nil, path)
	m.width, m.height = 110, 40

	// 'a' opens the add form.
	m = sendKey(t, m, "a")
	if m.mode != modeAddForm {
		t.Fatalf("expected add-form mode, got %v", m.mode)
	}

	// Fill required fields directly on the form inputs.
	m.form.inputs[fieldName].SetValue("prod")
	m.form.inputs[fieldHost].SetValue("1.2.3.4")
	m.form.inputs[fieldUser].SetValue("root")

	// enter on the Save button submits.
	m.form.focus = fieldSave
	m = sendKey(t, m, "enter")
	if m.mode != modeNormal {
		t.Fatalf("expected normal mode after submit, got %v (err=%q)", m.mode, m.form.errMsg)
	}
	if len(m.servers) != 1 || m.servers[0].Name != "prod" || m.servers[0].Port != 22 {
		t.Fatalf("server not added correctly: %+v", m.servers)
	}
	if m.selectedServer != 0 {
		t.Errorf("selected server = %d, want 0", m.selectedServer)
	}

	// Persisted to disk and reloadable.
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Servers) != 1 || loaded.Servers[0].Name != "prod" {
		t.Errorf("persisted config wrong: %+v", loaded.Servers)
	}
}

func TestAddServerInvalidStaysInForm(t *testing.T) {
	m := newModel(nil, nil, filepath.Join(t.TempDir(), "config.yaml"))
	m.width, m.height = 110, 40
	m = sendKey(t, m, "a")
	// name only, missing host/user -> validation error
	m.form.inputs[fieldName].SetValue("x")
	m.form.focus = fieldSave
	m = sendKey(t, m, "enter")

	if m.mode != modeAddForm {
		t.Fatalf("expected to stay in form, got mode %v", m.mode)
	}
	if m.form.errMsg == "" {
		t.Error("expected validation error message")
	}
	if len(m.servers) != 0 {
		t.Errorf("no server should be added: %+v", m.servers)
	}
}

func TestAddFormEscCancels(t *testing.T) {
	m := newModel(nil, nil, "")
	m.width, m.height = 110, 40
	m = sendKey(t, m, "a")
	m = sendKey(t, m, "esc")
	if m.mode != modeNormal {
		t.Errorf("esc should return to normal mode, got %v", m.mode)
	}
	if len(m.servers) != 0 {
		t.Errorf("no server should be added on cancel")
	}
}

func TestFormEnterAdvancesFields(t *testing.T) {
	m := newModel(nil, nil, "")
	m.width, m.height = 110, 40
	m = sendKey(t, m, "a")
	if m.form.focus != fieldName {
		t.Fatalf("expected initial focus on name, got %d", m.form.focus)
	}
	// enter on a non-last field advances rather than submitting.
	m = sendKey(t, m, "enter")
	if m.mode != modeAddForm {
		t.Fatalf("enter on first field should not submit, mode=%v", m.mode)
	}
	if m.form.focus != fieldHost {
		t.Errorf("enter should advance to host field, got %d", m.form.focus)
	}
}

func TestFormEnterOnDockerAdvancesToSave(t *testing.T) {
	m := newModel(nil, nil, "")
	m.width, m.height = 110, 40
	m = sendKey(t, m, "a")
	m.form.focus = fieldDocker
	m = sendKey(t, m, "enter")
	if m.mode != modeAddForm {
		t.Fatalf("enter on Docker should not submit, mode=%v", m.mode)
	}
	if m.form.focus != fieldSave {
		t.Errorf("enter on Docker should advance to Save, focus=%d", m.form.focus)
	}
}

func TestAutoRefreshCyclesPresets(t *testing.T) {
	m := testModel()
	if m.autoRefreshLabel() != "off" {
		t.Fatalf("auto-refresh should start off, got %q", m.autoRefreshLabel())
	}
	m = sendKey(t, m, "A")
	if m.autoIdx != 1 || m.tickGen != 1 {
		t.Errorf("first toggle: autoIdx=%d tickGen=%d, want 1/1", m.autoIdx, m.tickGen)
	}
	if m.autoRefreshLabel() != "5s" {
		t.Errorf("label after first toggle = %q, want 5s", m.autoRefreshLabel())
	}
	// Cycle all the way back to off.
	for i := 0; i < len(autoIntervals)-1; i++ {
		m = sendKey(t, m, "A")
	}
	if m.autoIdx != 0 || m.autoRefreshLabel() != "off" {
		t.Errorf("cycling should return to off, autoIdx=%d label=%q", m.autoIdx, m.autoRefreshLabel())
	}
}

func TestAutoRefreshTickIgnoresStaleGen(t *testing.T) {
	m := testModel()
	m.autoIdx = 1 // 5s, enabled
	m.tickGen = 5

	// A stale tick (old generation) must not mark servers loading.
	next, _ := m.Update(tickMsg{gen: 4})
	m = next.(AppModel)
	if m.loading["prod"] {
		t.Error("stale tick should be ignored")
	}

	// A current tick marks every server loading and re-arms the ticker.
	next, cmd := m.Update(tickMsg{gen: 5})
	m = next.(AppModel)
	if !m.loading["prod"] || !m.loading["staging"] {
		t.Errorf("current tick should mark servers loading: %+v", m.loading)
	}
	if cmd == nil {
		t.Error("current tick should return a refresh+reschedule command")
	}
}

func TestEditServerFlowPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	servers := []model.Server{
		{Name: "prod", Host: "1.2.3.4", Port: 22, User: "root", Docker: true},
		{Name: "staging", Host: "h2", Port: 22, User: "deploy", Docker: true},
	}
	if err := config.Save(path, &config.Config{Servers: servers}); err != nil {
		t.Fatal(err)
	}
	m := newModel(servers, nil, path)
	m.width, m.height = 110, 40
	m.selectedServer = 0

	// 'E' opens the edit form pre-filled with the selected server.
	m = sendKey(t, m, "E")
	if m.mode != modeEditForm {
		t.Fatalf("expected edit-form mode, got %v", m.mode)
	}
	if !m.form.editing || m.form.origName != "prod" {
		t.Fatalf("edit form not initialised: editing=%v origName=%q", m.form.editing, m.form.origName)
	}
	if got := m.form.inputs[fieldHost].Value(); got != "1.2.3.4" {
		t.Errorf("host not pre-filled, got %q", got)
	}

	// Rename and save (enter on the Save button).
	m.form.inputs[fieldName].SetValue("prod2")
	m.form.focus = fieldSave
	m = sendKey(t, m, "enter")
	if m.mode != modeNormal {
		t.Fatalf("expected normal mode after save, got %v (err=%q)", m.mode, m.form.errMsg)
	}

	// Renamed in place (still first), persisted, stale snapshot dropped.
	if m.servers[0].Name != "prod2" {
		t.Errorf("server not renamed in place: %+v", m.servers)
	}
	if m.selectedServer != 0 {
		t.Errorf("selection should follow edited server, got %d", m.selectedServer)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Servers) != 2 || loaded.Servers[0].Name != "prod2" {
		t.Errorf("edit not persisted: %+v", loaded.Servers)
	}
}

func TestDeleteFlowPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	servers := []model.Server{
		{Name: "prod", Host: "h1", Port: 22, User: "root", Docker: true},
		{Name: "staging", Host: "h2", Port: 22, User: "deploy", Docker: true},
	}
	if err := config.Save(path, &config.Config{Servers: servers}); err != nil {
		t.Fatal(err)
	}
	m := newModel(servers, nil, path)
	m.width, m.height = 110, 40
	m.selectedServer = 0

	// 'd' asks for confirmation.
	m = sendKey(t, m, "d")
	if m.mode != modeConfirmDelete {
		t.Fatalf("expected confirm mode, got %v", m.mode)
	}
	if strings.Contains(m.View(), "Delete server") == false {
		t.Errorf("confirm prompt not shown:\n%s", m.View())
	}

	// 'y' confirms.
	m = sendKey(t, m, "y")
	if m.mode != modeNormal {
		t.Fatalf("expected normal mode after delete, got %v", m.mode)
	}
	if len(m.servers) != 1 || m.servers[0].Name != "staging" {
		t.Fatalf("after delete: %+v", m.servers)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Servers) != 1 || loaded.Servers[0].Name != "staging" {
		t.Errorf("persisted after delete wrong: %+v", loaded.Servers)
	}
}

func TestDeleteCancel(t *testing.T) {
	servers := []model.Server{{Name: "prod", Host: "h", Port: 22, User: "u", Docker: true}}
	m := newModel(servers, nil, filepath.Join(t.TempDir(), "c.yaml"))
	m.width, m.height = 110, 40
	m = sendKey(t, m, "d")
	m = sendKey(t, m, "n")
	if m.mode != modeNormal {
		t.Errorf("n should cancel, got mode %v", m.mode)
	}
	if len(m.servers) != 1 {
		t.Errorf("server should remain: %+v", m.servers)
	}
}

func TestServerActionsGatedToServerFocus(t *testing.T) {
	servers := []model.Server{{Name: "prod", Host: "h", Port: 22, User: "u", Docker: true}}
	m := newModel(servers, nil, filepath.Join(t.TempDir(), "c.yaml"))
	m.width, m.height = 110, 40
	m.focus = FocusContainers

	// 'a' (add server) is a server action: ignored while containers are focused.
	m = sendKey(t, m, "a")
	if m.mode != modeNormal {
		t.Errorf("add should be ignored when containers focused, got mode %v", m.mode)
	}
	// 'd' (delete server) likewise.
	m = sendKey(t, m, "d")
	if m.mode != modeNormal {
		t.Errorf("delete should be ignored when containers focused, got mode %v", m.mode)
	}
}

func TestContainerActionsGatedToContainerFocus(t *testing.T) {
	servers := []model.Server{{Name: "prod", Host: "h", Port: 22, User: "u", Docker: true}}
	m := newModel(servers, nil, filepath.Join(t.TempDir(), "c.yaml"))
	m.width, m.height = 110, 40
	m.snapshots["prod"] = model.ServerSnapshot{
		Server:     servers[0],
		Online:     true,
		Containers: []model.Container{{Name: "web", ID: "abc", State: "running"}},
	}
	// Servers focused: container exec/logs are no-ops (no panic, mode unchanged,
	// no command). With a nil runner, openContainerShell would early-return
	// anyway, so gating is what keeps the dispatch from reaching it.
	if m.focus != FocusServers {
		t.Fatalf("expected default servers focus, got %v", m.focus)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd != nil {
		t.Errorf("exec should be ignored (no command) when servers focused")
	}
}

func TestTabTogglesFocus(t *testing.T) {
	servers := []model.Server{{Name: "prod", Host: "h", Port: 22, User: "u", Docker: true}}
	m := newModel(servers, nil, filepath.Join(t.TempDir(), "c.yaml"))
	m.width, m.height = 110, 40
	if m.focus != FocusServers {
		t.Fatalf("expected initial servers focus, got %v", m.focus)
	}
	m = sendKey(t, m, "tab")
	if m.focus != FocusContainers {
		t.Errorf("tab should switch to containers, got %v", m.focus)
	}
	m = sendKey(t, m, "tab")
	if m.focus != FocusServers {
		t.Errorf("tab should switch back to servers, got %v", m.focus)
	}
}
