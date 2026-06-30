// Package sshcmd runs commands on remote servers via the local `ssh` CLI.
//
// It deliberately shells out to /usr/bin/ssh (no golang.org/x/crypto/ssh) so it
// reuses the user's existing ssh config, keys, and ssh-agent. Non-interactive
// commands go through Run; interactive shells suspend the TUI and inherit stdio.
package sshcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/Vrex123/pharos/internal/model"
)

// Runner executes commands against a server.
type Runner interface {
	// Run executes command on the server non-interactively and returns stdout.
	Run(ctx context.Context, server model.Server, command string) (string, error)
	// InteractiveSSH opens a login shell on the server with inherited stdio.
	InteractiveSSH(server model.Server) error
	// InteractiveContainerShell opens a shell inside a Docker container.
	InteractiveContainerShell(server model.Server, container string) error
	// InteractiveContainerLogs follows a Docker container's logs.
	InteractiveContainerLogs(server model.Server, container string) error
}

// containerNameRe guards against shell injection in container names.
var containerNameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// ErrInvalidContainerName is returned when a container name fails validation.
var ErrInvalidContainerName = errors.New("invalid container name")

// ExecRunner is the default Runner backed by the local ssh binary.
//
// controlDir, when non-empty, holds a per-session directory for OpenSSH
// connection-multiplexing sockets (ControlMaster). It lets a single
// interactively-authenticated connection (e.g. a password typed once via
// Connect) be reused by every later non-interactive command, so pharos never
// sees or stores the password. It is empty on Windows, whose OpenSSH does not
// support ControlMaster multiplexing.
type ExecRunner struct {
	controlDir string
}

// New returns the default ExecRunner, creating a per-session control directory
// for connection multiplexing. On Windows (no ControlMaster support) or if the
// temp dir can't be created, controlDir stays empty and multiplexing is off.
func New() *ExecRunner {
	r := &ExecRunner{}
	if runtime.GOOS == "windows" {
		return r
	}
	if dir, err := os.MkdirTemp("", "pharos-cm-"); err == nil {
		r.controlDir = dir
	}
	return r
}

// muxArgs returns the ssh connection-multiplexing options, or nil when
// multiplexing is unavailable (Windows or no control dir). ControlMaster=auto
// reuses an existing master socket or becomes one; ControlPath uses %C (a short
// hash of host/port/user) to keep the socket path under the ~104-char Unix
// socket limit; ControlPersist keeps the master alive briefly after idle.
func (r *ExecRunner) muxArgs() []string {
	if r.controlDir == "" {
		return nil
	}
	return []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + filepath.Join(r.controlDir, "%C"),
		"-o", "ControlPersist=600",
	}
}

// MultiplexingEnabled reports whether connection multiplexing is available.
// When false (Windows), reusing a password-authenticated connection for
// background polling is not possible.
func (r *ExecRunner) MultiplexingEnabled() bool { return r.controlDir != "" }

// Disconnect tears down the multiplexed master connection to server, if any.
// Best-effort: errors (e.g. no master exists) are ignored.
func (r *ExecRunner) Disconnect(server model.Server) {
	if r.controlDir == "" {
		return
	}
	args := []string{
		"-o", "ControlPath=" + filepath.Join(r.controlDir, "%C"),
		"-O", "exit",
		"-p", strconv.Itoa(server.Port),
		target(server),
	}
	_ = exec.Command("ssh", args...).Run()
}

// Close removes the per-session control directory. Call on shutdown after
// Disconnecting any active masters.
func (r *ExecRunner) Close() {
	if r.controlDir != "" {
		_ = os.RemoveAll(r.controlDir)
	}
}

// hintForStderr returns a short, actionable message when ssh stderr indicates a
// private key it refused to use because its file permissions are too open
// (common on Windows, where OpenSSH ignores such keys). It returns "" when the
// signature is absent so callers keep the raw stderr.
func hintForStderr(stderr string) string {
	s := strings.ToLower(stderr)
	if strings.Contains(s, "unprotected private key") ||
		strings.Contains(s, "bad permissions") ||
		strings.Contains(s, "are too open") ||
		strings.Contains(s, "this private key will be ignored") {
		return "private key permissions too open; ssh ignored the key. " +
			"On Windows run: icacls \"<key>\" /inheritance:r /grant:r \"%USERNAME%:R\", " +
			"or add the key to ssh-agent. See the Windows troubleshooting in the README."
	}
	return ""
}

// baseArgs builds the common ssh arguments for a server, including connection
// multiplexing. When batch is true it adds BatchMode=yes, which forbids all
// interactive prompts (used for background commands so they never hang on a
// password prompt); interactive callers pass false so ssh can prompt for a
// password. extra is appended after the user@host target by the caller.
func (r *ExecRunner) baseArgs(server model.Server, batch bool) []string {
	args := []string{
		"-p", strconv.Itoa(server.Port),
		"-o", "ConnectTimeout=5",
	}
	if batch {
		args = append(args, "-o", "BatchMode=yes")
	}
	args = append(args, r.muxArgs()...)
	if server.IdentityFile != "" {
		args = append(args, "-i", server.IdentityFile)
	}
	return args
}

// target returns the user@host string.
func target(server model.Server) string {
	return server.User + "@" + server.Host
}

// runArgs builds the full ssh argument list for a non-interactive command.
func (r *ExecRunner) runArgs(server model.Server, command string) []string {
	args := r.baseArgs(server, true)
	args = append(args, target(server), command)
	return args
}

// Run executes command via `ssh ... user@host command`.
func (r *ExecRunner) Run(ctx context.Context, server model.Server, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "ssh", r.runArgs(server, command)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout.String(), fmt.Errorf("ssh timeout to %s", server.Name)
		}
		msg := stderr.String()
		if hint := hintForStderr(msg); hint != "" {
			msg = hint
		} else if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("ssh %s: %s", server.Name, msg)
	}
	return stdout.String(), nil
}

// InteractiveSSH runs `ssh user@host` as a foreground process with inherited
// stdio. Caller is responsible for suspending/resuming the TUI.
func (r *ExecRunner) InteractiveSSH(server model.Server) error {
	cmd := r.SSHCommand(server)
	return cmd.Run()
}

// SSHCommand returns the interactive ssh *exec.Cmd wired to the process stdio.
// Exposed so the TUI can hand it to tea.ExecProcess. BatchMode is off so ssh
// can prompt for a password; with ControlMaster=auto this connection becomes
// the reusable master.
func (r *ExecRunner) SSHCommand(server model.Server) *exec.Cmd {
	args := append(r.baseArgs(server, false), target(server))
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}

// ConnectCommand returns an interactive ssh command that establishes a
// multiplexed master connection and backgrounds itself after authentication
// (-f -N). BatchMode is off so ssh can prompt for a password in the foreground
// (the TUI must be suspended); once authenticated it forks to the background
// holding the master socket, and tea.ExecProcess returns. The password is
// entered into ssh's own prompt and never seen or stored by pharos.
func (r *ExecRunner) ConnectCommand(server model.Server) *exec.Cmd {
	args := append(r.baseArgs(server, false), "-f", "-N", target(server))
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}

// InteractiveContainerShell opens a shell inside a container via docker exec.
func (r *ExecRunner) InteractiveContainerShell(server model.Server, container string) error {
	cmd, err := r.ContainerShellCommand(server, container)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// ContainerShellCommand returns the interactive `ssh -t ... docker exec` command.
// It prefers sh, falling back to bash. The container name is validated to
// prevent shell injection in the remote command string.
func (r *ExecRunner) ContainerShellCommand(server model.Server, container string) (*exec.Cmd, error) {
	if !containerNameRe.MatchString(container) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidContainerName, container)
	}
	// Prefer sh: alpine/distroless images often lack bash.
	remote := fmt.Sprintf("docker exec -it %s sh", container)
	args := append(r.baseArgs(server, false), "-t", target(server), remote)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd, nil
}

// InteractiveContainerLogs follows a container's logs via docker logs -f.
func (r *ExecRunner) InteractiveContainerLogs(server model.Server, container string) error {
	cmd, err := r.ContainerLogsCommand(server, container)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// ContainerLogsCommand returns the interactive `ssh -t ... docker logs --tail
// 200 -f <container>` command. The container name is validated to prevent shell
// injection in the remote command string. The follow stream is exited with
// Ctrl+C.
func (r *ExecRunner) ContainerLogsCommand(server model.Server, container string) (*exec.Cmd, error) {
	if !containerNameRe.MatchString(container) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidContainerName, container)
	}
	remote := fmt.Sprintf("docker logs --tail 200 -f %s", container)
	args := append(r.baseArgs(server, false), "-t", target(server), remote)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd, nil
}

// ValidContainerName reports whether name is safe to use in a remote command.
func ValidContainerName(name string) bool {
	return containerNameRe.MatchString(name)
}
