// Package config loads, validates, and normalizes the pharos YAML config.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Vrex123/pharos/internal/model"
	"gopkg.in/yaml.v3"
)

// Config is the top-level config file structure.
type Config struct {
	Servers []model.Server
}

// rawServer mirrors the YAML schema. Docker is a pointer so we can tell
// "absent" (default true) apart from an explicit `docker: false`.
type rawServer struct {
	Name         string `yaml:"name"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	IdentityFile string `yaml:"identity_file"`
	Docker       *bool  `yaml:"docker"`
}

type rawConfig struct {
	Servers []rawServer `yaml:"servers"`
}

// DefaultPath returns the default config location: ~/.config/pharos/config.yaml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".config", "pharos", "config.yaml")
}

// Load reads and validates the config at path. A missing file is not an error:
// it yields an empty config so the app can start with no servers and let the
// user add them from the UI.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	return Parse(data)
}

// Parse decodes config bytes, applies defaults, and validates them. An empty
// server list is valid.
func Parse(data []byte) (*Config, error) {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg := &Config{}
	seen := make(map[string]bool, len(raw.Servers))
	for i, rs := range raw.Servers {
		s, err := normalize(rs)
		if err != nil {
			return nil, fmt.Errorf("server #%d: %w", i+1, err)
		}
		if seen[s.Name] {
			return nil, fmt.Errorf("duplicate server name %q", s.Name)
		}
		seen[s.Name] = true
		cfg.Servers = append(cfg.Servers, s)
	}
	return cfg, nil
}

// normalize converts a rawServer (YAML) to a validated model.Server, applying
// the docker default (true when absent) before sharing validation with
// NormalizeServer.
func normalize(rs rawServer) (model.Server, error) {
	docker := true
	if rs.Docker != nil {
		docker = *rs.Docker
	}
	return NormalizeServer(model.Server{
		Name:         rs.Name,
		Host:         rs.Host,
		Port:         rs.Port,
		User:         rs.User,
		IdentityFile: rs.IdentityFile,
		Docker:       docker,
	})
}

// NormalizeServer validates a single server, applies the port default (22), and
// expands ~ in IdentityFile. The Docker field is taken as-is; callers building a
// server from scratch should set the desired value (default true).
func NormalizeServer(s model.Server) (model.Server, error) {
	if s.Name == "" {
		return model.Server{}, fmt.Errorf("missing required field: name")
	}
	if s.Host == "" {
		return model.Server{}, fmt.Errorf("missing required field: host (server %q)", s.Name)
	}
	if s.User == "" {
		return model.Server{}, fmt.Errorf("missing required field: user (server %q)", s.Name)
	}

	if s.Port == 0 {
		s.Port = 22
	}

	identity, err := expandHome(s.IdentityFile)
	if err != nil {
		return model.Server{}, err
	}
	s.IdentityFile = identity

	return s, nil
}

// Add validates and appends a server, rejecting duplicate names.
func (c *Config) Add(s model.Server) error {
	ns, err := NormalizeServer(s)
	if err != nil {
		return err
	}
	for _, ex := range c.Servers {
		if ex.Name == ns.Name {
			return fmt.Errorf("duplicate server name %q", ns.Name)
		}
	}
	c.Servers = append(c.Servers, ns)
	return nil
}

// Update validates s, then replaces the server currently named origName,
// preserving its position. It rejects a name collision with a *different*
// server and errors if origName is not found.
func (c *Config) Update(origName string, s model.Server) error {
	ns, err := NormalizeServer(s)
	if err != nil {
		return err
	}
	idx := -1
	for i, ex := range c.Servers {
		if ex.Name == origName {
			idx = i
			continue
		}
		if ex.Name == ns.Name {
			return fmt.Errorf("duplicate server name %q", ns.Name)
		}
	}
	if idx == -1 {
		return fmt.Errorf("server %q not found", origName)
	}
	c.Servers[idx] = ns
	return nil
}

// Remove deletes the server with the given name, reporting whether it existed.
func (c *Config) Remove(name string) bool {
	for i, s := range c.Servers {
		if s.Name == name {
			c.Servers = append(c.Servers[:i], c.Servers[i+1:]...)
			return true
		}
	}
	return false
}

// Save writes the config back to path as YAML, creating parent directories as
// needed. It overwrites the file: comments in the original are not preserved and
// identity_file paths are written in their expanded form.
func Save(path string, c *Config) error {
	raw := rawConfig{Servers: make([]rawServer, 0, len(c.Servers))}
	for _, s := range c.Servers {
		docker := s.Docker
		raw.Servers = append(raw.Servers, rawServer{
			Name:         s.Name,
			Host:         s.Host,
			Port:         s.Port,
			User:         s.User,
			IdentityFile: s.IdentityFile,
			Docker:       &docker,
		})
	}

	data, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config dir %q: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

// expandHome expands a leading ~ in a path to the user's home directory.
func expandHome(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
	}
	return path, nil
}
