package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Vrex123/pharos/internal/model"
)

func TestParseValid(t *testing.T) {
	data := []byte(`
servers:
  - name: prod
    host: 1.2.3.4
    user: root
  - name: staging
    host: staging.example.com
    port: 2222
    user: deploy
    docker: false
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(cfg.Servers))
	}

	prod := cfg.Servers[0]
	if prod.Port != 22 {
		t.Errorf("prod port = %d, want default 22", prod.Port)
	}
	if !prod.Docker {
		t.Errorf("prod docker = false, want default true")
	}

	staging := cfg.Servers[1]
	if staging.Port != 2222 {
		t.Errorf("staging port = %d, want 2222", staging.Port)
	}
	if staging.Docker {
		t.Errorf("staging docker = true, want explicit false")
	}
}

func TestParseMissingFields(t *testing.T) {
	cases := map[string]string{
		"missing name": "servers:\n  - host: h\n    user: u\n",
		"missing host": "servers:\n  - name: n\n    user: u\n",
		"missing user": "servers:\n  - name: n\n    host: h\n",
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(data)); err == nil {
				t.Fatalf("expected error for %s", name)
			}
		})
	}
}

func TestParseDuplicateName(t *testing.T) {
	data := []byte(`
servers:
  - name: dup
    host: a
    user: u
  - name: dup
    host: b
    user: u
`)
	_, err := Parse(data)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestParseEmptyServersAllowed(t *testing.T) {
	cfg, err := Parse([]byte("servers: []\n"))
	if err != nil {
		t.Fatalf("empty servers should be valid, got %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Fatalf("got %d servers, want 0", len(cfg.Servers))
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if cfg == nil || len(cfg.Servers) != 0 {
		t.Fatalf("expected empty config, got %+v", cfg)
	}
}

func TestConfigAdd(t *testing.T) {
	c := &Config{}
	// Port left zero -> defaulted to 22; Docker is taken as-is (caller sets it).
	if err := c.Add(model.Server{Name: "a", Host: "h", User: "u", Docker: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(c.Servers) != 1 || c.Servers[0].Port != 22 || !c.Servers[0].Docker {
		t.Errorf("defaults not applied: %+v", c.Servers)
	}
	// missing required field
	if err := c.Add(model.Server{Name: "b", Host: "h"}); err == nil {
		t.Error("expected error for missing user")
	}
	// duplicate name
	if err := c.Add(model.Server{Name: "a", Host: "h2", User: "u2"}); err == nil {
		t.Error("expected duplicate name error")
	}
	if len(c.Servers) != 1 {
		t.Errorf("invalid adds should not append: %+v", c.Servers)
	}
}

func TestConfigUpdate(t *testing.T) {
	c := &Config{Servers: []model.Server{
		{Name: "a", Host: "h1", Port: 22, User: "u"},
		{Name: "b", Host: "h2", Port: 22, User: "u"},
	}}

	// Edit in place, including a rename; position preserved, defaults applied.
	if err := c.Update("a", model.Server{Name: "a2", Host: "h1b", User: "root", Docker: true}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if c.Servers[0].Name != "a2" || c.Servers[0].Host != "h1b" || c.Servers[0].Port != 22 {
		t.Errorf("update not applied in place: %+v", c.Servers)
	}

	// Renaming onto an existing different server is rejected.
	if err := c.Update("a2", model.Server{Name: "b", Host: "h", User: "u"}); err == nil {
		t.Error("expected duplicate name error")
	}

	// Keeping the same name is allowed (no self-collision).
	if err := c.Update("b", model.Server{Name: "b", Host: "h2b", User: "u"}); err != nil {
		t.Errorf("same-name update rejected: %v", err)
	}

	// Unknown origName is an error.
	if err := c.Update("missing", model.Server{Name: "x", Host: "h", User: "u"}); err == nil {
		t.Error("expected error for unknown server")
	}

	// Validation errors propagate.
	if err := c.Update("a2", model.Server{Name: "a2", Host: ""}); err == nil {
		t.Error("expected validation error for missing host")
	}
}

func TestConfigRemove(t *testing.T) {
	c := &Config{Servers: []model.Server{{Name: "a"}, {Name: "b"}}}
	if !c.Remove("a") {
		t.Error("expected Remove to report success")
	}
	if len(c.Servers) != 1 || c.Servers[0].Name != "b" {
		t.Errorf("after remove: %+v", c.Servers)
	}
	if c.Remove("missing") {
		t.Error("Remove of absent server should report false")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml") // sub dir must be created
	in := &Config{Servers: []model.Server{
		{Name: "prod", Host: "1.2.3.4", Port: 22, User: "root", Docker: true},
		{Name: "staging", Host: "h", Port: 2222, User: "deploy", IdentityFile: "/keys/id", Docker: false},
	}}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(out.Servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(out.Servers))
	}
	if out.Servers[1].Port != 2222 || out.Servers[1].Docker {
		t.Errorf("staging round-trip wrong: %+v", out.Servers[1])
	}
	if !out.Servers[0].Docker {
		t.Errorf("prod docker should be true: %+v", out.Servers[0])
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	data := []byte("servers:\n  - name: n\n    host: h\n    user: u\n    identity_file: ~/.ssh/id_ed25519\n")
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := filepath.Join(home, ".ssh", "id_ed25519")
	if got := cfg.Servers[0].IdentityFile; got != want {
		t.Errorf("identity_file = %q, want %q", got, want)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("servers:\n  - name: n\n    host: h\n    user: u\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(cfg.Servers))
	}
}
