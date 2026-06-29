package tui

import (
	"strings"
	"testing"

	"github.com/Vrex123/pharos/internal/model"
)

func TestAddFormValue(t *testing.T) {
	f := newAddForm()
	f.inputs[fieldName].SetValue("prod")
	f.inputs[fieldHost].SetValue("1.2.3.4")
	f.inputs[fieldPort].SetValue("2222")
	f.inputs[fieldUser].SetValue("root")
	f.inputs[fieldIdentity].SetValue("/keys/id")
	f.docker = false

	s, err := f.value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	if s.Name != "prod" || s.Host != "1.2.3.4" || s.Port != 2222 || s.User != "root" {
		t.Errorf("server = %+v", s)
	}
	if s.IdentityFile != "/keys/id" || s.Docker {
		t.Errorf("identity/docker = %q/%v", s.IdentityFile, s.Docker)
	}
}

func TestAddFormDefaultsAndBadPort(t *testing.T) {
	f := newAddForm()
	if !f.docker {
		t.Error("docker should default to true")
	}
	// empty port -> 0 (config.NormalizeServer applies 22 later)
	s, err := f.value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	if s.Port != 0 {
		t.Errorf("empty port should be 0, got %d", s.Port)
	}

	f.inputs[fieldPort].SetValue("notaport")
	if _, err := f.value(); err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestAddFormView(t *testing.T) {
	f := newAddForm()
	out := f.View()
	for _, want := range []string{"Add Server", "Name:", "Host:", "Docker:", "[x]", "[ Save ]", "esc cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("form view missing %q", want)
		}
	}
}

func TestNewEditFormPrefills(t *testing.T) {
	s := model.Server{Name: "prod", Host: "1.2.3.4", Port: 2222, User: "root", IdentityFile: "/keys/id", Docker: false}
	f := newEditForm(s)
	if !f.editing || f.origName != "prod" {
		t.Fatalf("editing=%v origName=%q", f.editing, f.origName)
	}
	if f.inputs[fieldName].Value() != "prod" || f.inputs[fieldHost].Value() != "1.2.3.4" ||
		f.inputs[fieldPort].Value() != "2222" || f.inputs[fieldUser].Value() != "root" ||
		f.inputs[fieldIdentity].Value() != "/keys/id" {
		t.Errorf("inputs not pre-filled: %+v", f.inputs)
	}
	if f.docker {
		t.Error("docker should be false")
	}
	if !strings.Contains(f.View(), "Edit Server") {
		t.Error("edit form view should say Edit Server")
	}
}

func TestNewEditFormZeroPort(t *testing.T) {
	// Port 0 (unset) should leave the field empty, not render "0".
	f := newEditForm(model.Server{Name: "x", Host: "h", User: "u", Docker: true})
	if v := f.inputs[fieldPort].Value(); v != "" {
		t.Errorf("zero port should be empty, got %q", v)
	}
}

func TestFormNextFieldWraps(t *testing.T) {
	f := newAddForm()
	for i := 0; i < fieldCount-1; i++ {
		f, _ = f.nextField()
	}
	if f.focus != fieldSave {
		t.Fatalf("focus = %d, want fieldSave", f.focus)
	}
	f, _ = f.nextField()
	if f.focus != fieldName {
		t.Errorf("next from last field should wrap to name, got %d", f.focus)
	}
}
