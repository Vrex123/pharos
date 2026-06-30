package sshcmd

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Vrex123/pharos/internal/model"
)

func TestRunArgsNoIdentity(t *testing.T) {
	s := model.Server{Name: "prod", Host: "1.2.3.4", Port: 22, User: "root"}
	args := runArgs(s, "echo ok")

	if !hasPair(args, "-p", "22") {
		t.Errorf("missing -p 22 in %v", args)
	}
	if slices.Contains(args, "-i") {
		t.Errorf("unexpected -i without identity file: %v", args)
	}
	if args[len(args)-2] != "root@1.2.3.4" || args[len(args)-1] != "echo ok" {
		t.Errorf("target/command tail wrong: %v", args)
	}
}

func TestRunArgsWithIdentity(t *testing.T) {
	s := model.Server{Name: "s", Host: "h", Port: 2222, User: "deploy", IdentityFile: "/home/u/.ssh/id_ed25519"}
	args := runArgs(s, "uptime")

	if !hasPair(args, "-p", "2222") {
		t.Errorf("missing -p 2222 in %v", args)
	}
	if !hasPair(args, "-i", "/home/u/.ssh/id_ed25519") {
		t.Errorf("missing -i identity in %v", args)
	}
}

func TestRunTimeout(t *testing.T) {
	s := model.Server{Name: "s", Host: "192.0.2.1", Port: 22, User: "u"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := (&ExecRunner{}).Run(ctx, s, "echo ok")
	if err == nil {
		t.Fatal("expected error from unreachable host / timeout")
	}
}

func TestContainerShellCommandValidation(t *testing.T) {
	r := &ExecRunner{}
	s := model.Server{Name: "s", Host: "h", Port: 22, User: "u"}

	if _, err := r.ContainerShellCommand(s, "web_1.app-2"); err != nil {
		t.Errorf("valid name rejected: %v", err)
	}

	bad := []string{"web; rm -rf /", "$(whoami)", "a b", "`id`", ""}
	for _, name := range bad {
		if _, err := r.ContainerShellCommand(s, name); err == nil {
			t.Errorf("expected rejection for %q", name)
		}
	}
}

func TestContainerShellCommandUsesSh(t *testing.T) {
	r := &ExecRunner{}
	s := model.Server{Name: "s", Host: "h", Port: 22, User: "u"}
	cmd, err := r.ContainerShellCommand(s, "web")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "-t") {
		t.Errorf("expected -t (tty) flag: %v", cmd.Args)
	}
	if !strings.Contains(joined, "docker exec -it web sh") {
		t.Errorf("expected docker exec sh: %v", cmd.Args)
	}
}

func TestContainerLogsCommand(t *testing.T) {
	r := &ExecRunner{}
	s := model.Server{Name: "s", Host: "h", Port: 22, User: "u"}
	cmd, err := r.ContainerLogsCommand(s, "web")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "-t") {
		t.Errorf("expected -t (tty) flag: %v", cmd.Args)
	}
	if !strings.Contains(joined, "docker logs --tail 200 -f web") {
		t.Errorf("expected docker logs follow: %v", cmd.Args)
	}
	if _, err := r.ContainerLogsCommand(s, "web; rm -rf /"); err == nil {
		t.Error("expected rejection of unsafe container name")
	}
}

func TestValidContainerName(t *testing.T) {
	if !ValidContainerName("ok-name_1.2") {
		t.Error("expected valid")
	}
	if ValidContainerName("bad name") {
		t.Error("expected invalid")
	}
}

func TestHintForStderr(t *testing.T) {
	tests := []struct {
		name    string
		stderr  string
		wantApp bool // expect a non-empty hint
	}{
		{
			name:    "windows bad permissions",
			stderr:  "Bad permissions. Try removing permissions for user: DESKTOP-3QH61CR\\Val (S-1-5-21-1) on file C:/Users/Val/.ssh/id_rsa.\nThis private key will be ignored.",
			wantApp: true,
		},
		{
			name:    "unprotected warning",
			stderr:  "WARNING: UNPROTECTED PRIVATE KEY FILE!\nPermissions for 'id_rsa' are too open.",
			wantApp: true,
		},
		{
			name:    "unrelated error",
			stderr:  "ssh: connect to host 1.2.3.4 port 22: Connection timed out",
			wantApp: false,
		},
		{
			name:    "empty",
			stderr:  "",
			wantApp: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := hintForStderr(tt.stderr)
			if tt.wantApp {
				if hint == "" {
					t.Fatalf("expected hint, got empty for %q", tt.stderr)
				}
				if !strings.Contains(hint, "icacls") {
					t.Errorf("hint missing icacls guidance: %q", hint)
				}
			} else if hint != "" {
				t.Errorf("expected empty hint, got %q", hint)
			}
		})
	}
}

func hasPair(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}
