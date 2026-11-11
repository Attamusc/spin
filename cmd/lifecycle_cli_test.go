package cmd

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The helper accepts a fresh challenge only after the parent observes CLI exit.
func TestLifecycleCLIHelper(t *testing.T) {
	if os.Getenv("SPIN_LIFECYCLE_HELPER") != "1" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.WriteFile(os.Getenv("SPIN_LIFECYCLE_ADDRESS"), []byte(listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		challenge, err := bufio.NewReader(conn).ReadString('\n')
		if err == nil {
			fmt.Fprint(conn, "alive:"+challenge)
		}
		conn.Close()
	}
}

func TestUpChildSurvivesCLIExit(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "spin")
	build := exec.Command("go", "build", "-o", binary, "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	project := filepath.Join(home, "projects", "example.com", "owner", "repo")
	addressFile := filepath.Join(home, "address")
	configDir := filepath.Join(home, ".config", "spin")
	for _, dir := range []string{project, configDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(fmt.Sprintf("projects_root = %q\n", filepath.Join(home, "projects"))), 0600); err != nil {
		t.Fatal(err)
	}
	// Shell-quote the executable separately from TOML encoding the command.
	command := "exec '" + strings.ReplaceAll(helper, "'", "'\\''") + "' -test.run=^TestLifecycleCLIHelper$"
	config := fmt.Sprintf("[dev]\ncommand = \"true\"\n[live]\ncommand = %q\n[live.env]\nSPIN_LIFECYCLE_HELPER = \"1\"\nSPIN_LIFECYCLE_ADDRESS = %q\n", command, addressFile)
	if err := os.WriteFile(filepath.Join(project, ".spin.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
		return cmd.CombinedOutput()
	}
	pidPath := filepath.Join(home, "projects", ".spin", "owner", "repo", "live.pid")
	// Only this fixture's recorded group can be signaled, including on failed assertions.
	defer func() {
		if data, err := os.ReadFile(pidPath); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 1 {
				syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
	}()
	out, err := run("up")
	if err != nil {
		t.Fatalf("public up: %v: %s", err, out)
	}
	t.Logf("public CLI up exited successfully before child exchange: %s", strings.TrimSpace(string(out)))
	deadline := time.Now().Add(10 * time.Second)
	var address []byte
	for time.Now().Before(deadline) {
		address, err = os.ReadFile(addressFile)
		if err == nil && len(address) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil || len(address) == 0 {
		t.Fatalf("helper readiness: %v", err)
	}
	conn, err := net.DialTimeout("tcp", string(address), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	challenge := fmt.Sprintf("post-cli-exit-%d\n", time.Now().UnixNano())
	if _, err := fmt.Fprint(conn, challenge); err != nil {
		t.Fatal(err)
	}
	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || response != "alive:"+challenge {
		t.Fatalf("child response %q: %v", response, err)
	}
	t.Logf("detached application answered after CLI exit: %s", strings.TrimSpace(response))
	out, err = run("down")
	if err != nil {
		t.Fatalf("public down: %v: %s", err, out)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("down retained PID record: %v", err)
	}
	t.Logf("public CLI down exited successfully; PID record removed: %s", strings.TrimSpace(string(out)))
}
