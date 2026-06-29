// Package tui implements the Bubble Tea terminal UI for pharos.
package tui

import (
	"context"
	"time"

	"github.com/Vrex123/pharos/internal/collector"
	"github.com/Vrex123/pharos/internal/config"
	"github.com/Vrex123/pharos/internal/model"
	"github.com/Vrex123/pharos/internal/sshcmd"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// autoIntervals are the auto-refresh presets cycled by the auto-refresh key.
// Index 0 (zero duration) means auto-refresh is off.
var autoIntervals = []time.Duration{0, 5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second}

// FocusArea identifies which panel receives navigation keys.
type FocusArea int

const (
	FocusServers FocusArea = iota
	FocusContainers
)

// bottomTab identifies which view the (focusable) bottom panel is showing.
type bottomTab int

const (
	tabContainers bottomTab = iota
	tabProcesses
)

// mode is the top-level UI state: normal navigation, the add-server form, or the
// delete confirmation prompt.
type mode int

const (
	modeNormal mode = iota
	modeAddForm
	modeEditForm
	modeConfirmDelete
)

// AppModel is the root Bubble Tea model.
type AppModel struct {
	runner    sshcmd.Runner
	cfgPath   string
	servers   []model.Server
	snapshots map[string]model.ServerSnapshot
	loading   map[string]bool

	selectedServer    int
	selectedContainer int
	selectedProcess   int
	focus             FocusArea
	bottomTab         bottomTab

	mode mode
	form addForm

	// autoIdx indexes autoIntervals (0 = off). tickGen is bumped whenever
	// auto-refresh is toggled so stale ticks from a previous setting are ignored.
	autoIdx int
	tickGen int

	width, height int
	statusMsg     string
}

// snapshotMsg carries a freshly collected server snapshot.
type snapshotMsg struct{ snapshot model.ServerSnapshot }

// tickMsg is delivered by the auto-refresh ticker. gen identifies the
// auto-refresh generation it was scheduled under.
type tickMsg struct{ gen int }

// shellFinishedMsg is sent after an interactive shell returns.
type shellFinishedMsg struct {
	server model.Server
	err    error
}

// Run builds and runs the TUI program for the given config. cfgPath is where
// changes (added/removed servers) are persisted.
func Run(cfg *config.Config, cfgPath string) error {
	m := newModel(cfg.Servers, sshcmd.New(), cfgPath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(servers []model.Server, runner sshcmd.Runner, cfgPath string) AppModel {
	return AppModel{
		runner:    runner,
		cfgPath:   cfgPath,
		servers:   servers,
		snapshots: make(map[string]model.ServerSnapshot),
		loading:   make(map[string]bool),
		focus:     FocusServers,
		mode:      modeNormal,
	}
}

// Init kicks off a refresh of every server.
func (m AppModel) Init() tea.Cmd {
	return m.refreshAllCmd()
}

func (m AppModel) refreshServerCmd(s model.Server) tea.Cmd {
	return func() tea.Msg {
		snap := collector.CollectServer(context.Background(), m.runner, s)
		return snapshotMsg{snapshot: snap}
	}
}

// scheduleTick arms the next auto-refresh tick for the current interval,
// tagging it with the current generation so stale ticks can be discarded.
func (m AppModel) scheduleTick() tea.Cmd {
	gen := m.tickGen
	return tea.Tick(autoIntervals[m.autoIdx], func(time.Time) tea.Msg {
		return tickMsg{gen: gen}
	})
}

func (m AppModel) refreshAllCmd() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.servers))
	for _, s := range m.servers {
		cmds = append(cmds, m.refreshServerCmd(s))
	}
	return tea.Batch(cmds...)
}

func (m AppModel) selected() (model.Server, bool) {
	if m.selectedServer < 0 || m.selectedServer >= len(m.servers) {
		return model.Server{}, false
	}
	return m.servers[m.selectedServer], true
}

// Update handles incoming messages.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case snapshotMsg:
		name := msg.snapshot.Server.Name
		m.snapshots[name] = msg.snapshot
		m.loading[name] = false
		return m, nil

	case tickMsg:
		// Ignore stale ticks (auto-refresh toggled since this was scheduled).
		if msg.gen != m.tickGen || autoIntervals[m.autoIdx] == 0 {
			return m, nil
		}
		for _, s := range m.servers {
			m.loading[s.Name] = true
		}
		return m, tea.Batch(m.refreshAllCmd(), m.scheduleTick())

	case shellFinishedMsg:
		if msg.err != nil {
			m.statusMsg = "shell error: " + msg.err.Error()
		} else {
			m.statusMsg = ""
		}
		m.loading[msg.server.Name] = true
		return m, m.refreshServerCmd(msg.server)

	case tea.KeyMsg:
		switch m.mode {
		case modeAddForm, modeEditForm:
			return m.handleFormKey(msg)
		case modeConfirmDelete:
			return m.handleConfirmKey(msg)
		default:
			return m.handleKey(msg)
		}
	}
	return m, nil
}

// handleFormKey routes keys to the add/edit-server form. enter advances to the
// next field and submits only on the last field (Docker); esc cancels.
func (m AppModel) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "enter":
		if m.form.focus == fieldSave {
			return m.submitForm()
		}
		var cmd tea.Cmd
		m.form, cmd = m.form.nextField()
		return m, cmd
	}
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

// submitForm validates the form, adds or updates and persists the server, then
// refreshes it.
func (m AppModel) submitForm() (tea.Model, tea.Cmd) {
	s, err := m.form.value()
	if err != nil {
		m.form.errMsg = err.Error()
		return m, nil
	}
	cfg := &config.Config{Servers: m.servers}
	if m.form.editing {
		// Drop a stale snapshot if the server was renamed.
		if m.form.origName != s.Name {
			delete(m.snapshots, m.form.origName)
			delete(m.loading, m.form.origName)
		}
		if err := cfg.Update(m.form.origName, s); err != nil {
			m.form.errMsg = err.Error()
			return m, nil
		}
	} else if err := cfg.Add(s); err != nil {
		m.form.errMsg = err.Error()
		return m, nil
	}
	m.servers = cfg.Servers
	saved := serverByName(m.servers, s.Name)

	m.mode = modeNormal
	m.selectedServer = indexByName(m.servers, s.Name)
	m.focus = FocusServers
	if err := config.Save(m.cfgPath, cfg); err != nil {
		m.statusMsg = "saved in memory but not to disk: " + err.Error()
	} else {
		m.statusMsg = ""
	}
	m.loading[saved.Name] = true
	return m, m.refreshServerCmd(saved)
}

// serverByName returns the server with the given name (zero value if absent).
func serverByName(servers []model.Server, name string) model.Server {
	if i := indexByName(servers, name); i >= 0 {
		return servers[i]
	}
	return model.Server{}
}

// indexByName returns the index of the server with the given name, or -1.
func indexByName(servers []model.Server, name string) int {
	for i, s := range servers {
		if s.Name == name {
			return i
		}
	}
	return -1
}

// handleConfirmKey handles the delete confirmation prompt.
func (m AppModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		return m.deleteSelected()
	default: // n, esc, anything else cancels
		m.mode = modeNormal
		return m, nil
	}
}

// deleteSelected removes the selected server and persists the change.
func (m AppModel) deleteSelected() (tea.Model, tea.Cmd) {
	s, ok := m.selected()
	m.mode = modeNormal
	if !ok {
		return m, nil
	}
	cfg := &config.Config{Servers: m.servers}
	cfg.Remove(s.Name)
	m.servers = cfg.Servers
	delete(m.snapshots, s.Name)
	delete(m.loading, s.Name)
	if len(m.servers) == 0 {
		m.selectedServer = 0
	} else {
		m.selectedServer = clamp(m.selectedServer, 0, len(m.servers)-1)
	}
	m.selectedContainer = 0
	m.selectedProcess = 0
	if err := config.Save(m.cfgPath, cfg); err != nil {
		m.statusMsg = "removed in memory but not saved to disk: " + err.Error()
	} else {
		m.statusMsg = ""
	}
	return m, nil
}

// handleKey handles normal-mode keys: always-available navigation first, then
// the actions for whichever panel is focused.
func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Tab):
		if m.focus == FocusServers {
			m.focus = FocusContainers
		} else {
			m.focus = FocusServers
		}
		return m, nil

	case key.Matches(msg, keys.SwitchTab):
		// Cycle the bottom panel's tab (Containers <-> Processes).
		if m.bottomTab == tabContainers {
			m.bottomTab = tabProcesses
		} else {
			m.bottomTab = tabContainers
		}
		return m, nil

	case key.Matches(msg, keys.Up):
		m.moveSelection(-1)
		return m, nil

	case key.Matches(msg, keys.Down):
		m.moveSelection(1)
		return m, nil
	}

	switch m.focus {
	case FocusServers:
		return m.handleServerKey(msg)
	case FocusContainers:
		return m.handleContainerKey(msg)
	}
	return m, nil
}

// handleServerKey handles actions available when the Servers panel is focused.
func (m AppModel) handleServerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Refresh):
		if s, ok := m.selected(); ok {
			m.loading[s.Name] = true
			m.statusMsg = ""
			return m, m.refreshServerCmd(s)
		}
		return m, nil

	case key.Matches(msg, keys.RefreshAll):
		for _, s := range m.servers {
			m.loading[s.Name] = true
		}
		m.statusMsg = ""
		return m, m.refreshAllCmd()

	case key.Matches(msg, keys.Auto):
		// Cycle off -> 5s -> 15s -> 30s -> 60s -> off; bump the generation so
		// any in-flight tick from the previous setting is discarded.
		m.autoIdx = (m.autoIdx + 1) % len(autoIntervals)
		m.tickGen++
		if autoIntervals[m.autoIdx] == 0 {
			return m, nil
		}
		return m, m.scheduleTick()

	case key.Matches(msg, keys.SSH):
		return m.openSSH()

	case key.Matches(msg, keys.Add):
		m.mode = modeAddForm
		m.form = newAddForm()
		m.statusMsg = ""
		return m, textinput.Blink

	case key.Matches(msg, keys.Edit):
		if s, ok := m.selected(); ok {
			m.mode = modeEditForm
			m.form = newEditForm(s)
			m.statusMsg = ""
			return m, textinput.Blink
		}
		return m, nil

	case key.Matches(msg, keys.Delete):
		if _, ok := m.selected(); ok {
			m.mode = modeConfirmDelete
		}
		return m, nil
	}
	return m, nil
}

// handleContainerKey handles actions available when the bottom panel is focused.
// The Processes tab is read-only, so exec/logs apply only to the Containers tab.
func (m AppModel) handleContainerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.bottomTab != tabContainers {
		return m, nil
	}
	switch {
	case key.Matches(msg, keys.ContainerExec):
		return m.openContainerShell()

	case key.Matches(msg, keys.ContainerLogs):
		return m.openContainerLogs()
	}
	return m, nil
}

func (m *AppModel) moveSelection(delta int) {
	switch m.focus {
	case FocusServers:
		n := len(m.servers)
		if n == 0 {
			return
		}
		m.selectedServer = clamp(m.selectedServer+delta, 0, n-1)
		m.selectedContainer = 0
		m.selectedProcess = 0
	case FocusContainers:
		if m.bottomTab == tabProcesses {
			n := len(m.currentProcesses())
			if n == 0 {
				return
			}
			m.selectedProcess = clamp(m.selectedProcess+delta, 0, n-1)
			return
		}
		n := len(m.currentContainers())
		if n == 0 {
			return
		}
		m.selectedContainer = clamp(m.selectedContainer+delta, 0, n-1)
	}
}

func (m AppModel) currentContainers() []model.Container {
	s, ok := m.selected()
	if !ok {
		return nil
	}
	return m.snapshots[s.Name].Containers
}

func (m AppModel) currentProcesses() []model.Process {
	s, ok := m.selected()
	if !ok {
		return nil
	}
	return m.snapshots[s.Name].Processes
}

// openSSH suspends the TUI and opens an SSH shell on the selected server.
func (m AppModel) openSSH() (tea.Model, tea.Cmd) {
	s, ok := m.selected()
	if !ok {
		return m, nil
	}
	er, ok := m.runner.(*sshcmd.ExecRunner)
	if !ok {
		return m, nil
	}
	cmd := er.SSHCommand(s)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return shellFinishedMsg{server: s, err: err}
	})
}

// openContainerShell suspends the TUI and opens a shell in the selected container.
func (m AppModel) openContainerShell() (tea.Model, tea.Cmd) {
	s, ok := m.selected()
	if !ok {
		return m, nil
	}
	containers := m.currentContainers()
	if m.selectedContainer < 0 || m.selectedContainer >= len(containers) {
		return m, nil
	}
	name := containers[m.selectedContainer].Name
	er, ok := m.runner.(*sshcmd.ExecRunner)
	if !ok {
		return m, nil
	}
	cmd, err := er.ContainerShellCommand(s, name)
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return shellFinishedMsg{server: s, err: err}
	})
}

// openContainerLogs suspends the TUI and follows the selected container's logs.
func (m AppModel) openContainerLogs() (tea.Model, tea.Cmd) {
	s, ok := m.selected()
	if !ok {
		return m, nil
	}
	containers := m.currentContainers()
	if m.selectedContainer < 0 || m.selectedContainer >= len(containers) {
		return m, nil
	}
	name := containers[m.selectedContainer].Name
	er, ok := m.runner.(*sshcmd.ExecRunner)
	if !ok {
		return m, nil
	}
	cmd, err := er.ContainerLogsCommand(s, name)
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return shellFinishedMsg{server: s, err: err}
	})
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
