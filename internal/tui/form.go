package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Vrex123/pharos/internal/model"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Form field indices. fieldDocker is a bool toggle and fieldSave is a button,
// neither is a text input, so the text inputs slice has length fieldDocker.
const (
	fieldName = iota
	fieldHost
	fieldPort
	fieldUser
	fieldIdentity
	fieldDocker
	fieldSave
	fieldCount
)

var fieldLabels = []string{"Name", "Host", "Port", "User", "Identity"}

// addForm is the "add server" / "edit server" input form. When editing is true
// it edits the existing server named origName instead of adding a new one.
type addForm struct {
	inputs   []textinput.Model
	docker   bool
	focus    int
	errMsg   string
	editing  bool
	origName string
}

func newAddForm() addForm {
	f := addForm{docker: true}
	placeholders := []string{"prod", "1.2.3.4 or host", "22", "root", "~/.ssh/id_ed25519 (optional)"}
	f.inputs = make([]textinput.Model, fieldDocker)
	for i := range f.inputs {
		ti := textinput.New()
		ti.Placeholder = placeholders[i]
		ti.Prompt = ""
		ti.CharLimit = 256
		ti.Width = 36
		f.inputs[i] = ti
	}
	f.inputs[fieldName].Focus()
	return f
}

// newEditForm builds a form pre-filled with an existing server's values.
func newEditForm(s model.Server) addForm {
	f := newAddForm()
	f.editing = true
	f.origName = s.Name
	f.docker = s.Docker
	f.inputs[fieldName].SetValue(s.Name)
	f.inputs[fieldHost].SetValue(s.Host)
	if s.Port != 0 {
		f.inputs[fieldPort].SetValue(strconv.Itoa(s.Port))
	}
	f.inputs[fieldUser].SetValue(s.User)
	f.inputs[fieldIdentity].SetValue(s.IdentityFile)
	return f
}

// Update handles navigation and text entry within the form. enter/esc are
// handled by the parent model, not here.
func (f addForm) Update(msg tea.Msg) (addForm, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "tab", "down":
			return f.nextField()
		case "shift+tab", "up":
			return f.prevField()
		case " ":
			if f.focus == fieldDocker {
				f.docker = !f.docker
				return f, nil
			}
		}
	}

	if f.focus < len(f.inputs) {
		var cmd tea.Cmd
		f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
		return f, cmd
	}
	return f, nil
}

// nextField moves focus to the next field, wrapping around.
func (f addForm) nextField() (addForm, tea.Cmd) {
	f.focus = (f.focus + 1) % fieldCount
	return f, f.focusActive()
}

// prevField moves focus to the previous field, wrapping around.
func (f addForm) prevField() (addForm, tea.Cmd) {
	f.focus = (f.focus - 1 + fieldCount) % fieldCount
	return f, f.focusActive()
}

// focusActive focuses the currently selected text input and blurs the rest.
func (f addForm) focusActive() tea.Cmd {
	var cmd tea.Cmd
	for i := range f.inputs {
		if i == f.focus {
			cmd = f.inputs[i].Focus()
		} else {
			f.inputs[i].Blur()
		}
	}
	return cmd
}

// value builds a model.Server from the form. Defaults (port 22, ~ expansion)
// are applied later by config.NormalizeServer; here we only parse the port.
func (f addForm) value() (model.Server, error) {
	port := 0
	if ps := strings.TrimSpace(f.inputs[fieldPort].Value()); ps != "" {
		p, err := strconv.Atoi(ps)
		if err != nil || p < 1 || p > 65535 {
			return model.Server{}, fmt.Errorf("invalid port %q", ps)
		}
		port = p
	}
	return model.Server{
		Name:         strings.TrimSpace(f.inputs[fieldName].Value()),
		Host:         strings.TrimSpace(f.inputs[fieldHost].Value()),
		Port:         port,
		User:         strings.TrimSpace(f.inputs[fieldUser].Value()),
		IdentityFile: strings.TrimSpace(f.inputs[fieldIdentity].Value()),
		Docker:       f.docker,
	}, nil
}

func (f addForm) View() string {
	var b strings.Builder
	title := "Add Server"
	if f.editing {
		title = "Edit Server"
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	for i, ti := range f.inputs {
		marker := "  "
		if f.focus == i {
			marker = "> "
		}
		b.WriteString(fmt.Sprintf("%s%-10s %s\n", marker, fieldLabels[i]+":", ti.View()))
	}

	box := "[ ]"
	if f.docker {
		box = "[x]"
	}
	marker := "  "
	if f.focus == fieldDocker {
		marker = "> "
	}
	b.WriteString(fmt.Sprintf("%s%-10s %s %s\n", marker, "Docker:", box, mutedStyle.Render("(space toggles)")))

	saveMarker := "  "
	saveBtn := "[ Save ]"
	if f.focus == fieldSave {
		saveMarker = "> "
		saveBtn = selectedItemStyle.Render(" Save ")
	}
	b.WriteString("\n" + saveMarker + saveBtn + "\n")

	if f.errMsg != "" {
		b.WriteString("\n" + errStyle.Render("! "+f.errMsg))
	}
	b.WriteString("\n\n" + footerStyle.Render("tab/↑↓ move • space toggles Docker • enter on Save saves • esc cancel"))
	return b.String()
}
