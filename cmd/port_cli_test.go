package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/attamusc/spin/internal/process"
)

func TestExecutionPortCLI(t *testing.T) {
	binary := buildSpin(t)
	for _, tc := range []struct {
		name, command string
		dev, live     int
		wantExit      int
		wantRun       bool
	}{
		{name: "invalid dev", command: "dev", dev: 70000, live: 3000, wantExit: 1},
		{name: "invalid live", command: "up", dev: 3001, live: -1, wantExit: 1},
		{name: "valid dev ignores invalid live", command: "dev", dev: 3001, live: 70000, wantExit: 7, wantRun: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			project := filepath.Join(home, "projects", "example.com", "owner", "repo")
			marker := filepath.Join(home, "executed")
			command := "printf ran > \"$PORT_TEST_MARKER\"; exit 7"
			content := fmt.Sprintf("[dev]\ncommand=%q\nport=%d\n[dev.env]\nPORT_TEST_MARKER=%q\n[live]\ncommand=%q\nport=%d\n", command, tc.dev, marker, command, tc.live)
			writeStatusFixture(t, filepath.Join(project, ".spin.toml"), content)
			stdout, stderr, err := captureCLI(spinCommand(t, binary, home, project, tc.command))
			if exitCode(err) != tc.wantExit {
				t.Fatalf("exit=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
			_, markerErr := os.Stat(marker)
			if (markerErr == nil) != tc.wantRun {
				t.Fatalf("unexpected execution: %v", markerErr)
			}
			if !tc.wantRun {
				diagnostic := "invalid dev port"
				if tc.command == "up" {
					diagnostic = "invalid live port"
				}
				if !strings.Contains(stderr.String(), diagnostic) {
					t.Fatalf("missing port diagnostic: %q", stderr)
				}
				if _, err := os.Stat(filepath.Join(home, "projects", ".spin")); !os.IsNotExist(err) {
					t.Fatalf("invalid execution created state: %v", err)
				}
			}
		})
	}
}

func TestOpenRejectsInvalidPortAndStatusRetainsIdentity(t *testing.T) {
	binary := buildSpin(t)
	home := t.TempDir()
	project := filepath.Join(home, "projects", "example.com", "owner", "repo")
	writeStatusFixture(t, filepath.Join(project, ".spin.toml"), "[dev]\ncommand='true'\nport=3001\n[live]\ncommand='true'\nport=70000\n")
	pidFile := filepath.Join(home, "projects", ".spin", "owner", "repo", "live.pid")
	if err := process.WritePID(pidFile, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	marker := filepath.Join(home, "browser-opened")
	opener, err := openerForOS(runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tools, opener)
	writeStatusFixture(t, path, "#!/bin/sh\nprintf opened > \"$PORT_TEST_MARKER\"\n")
	if err := os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := spinCommand(t, binary, home, project, "open")
	cmd.Env = append(cmd.Env, "PATH="+tools, "PORT_TEST_MARKER="+marker)
	_, stderr, err := captureCLI(cmd)
	if exitCode(err) != 1 || !strings.Contains(stderr.String(), "invalid selected port") {
		t.Fatalf("invalid open: %v %q", err, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("invalid URL reached the opener: %v", err)
	}
	before := observationManifest(t, home)
	stdout, _, err := captureCLI(spinCommand(t, binary, home, project, "status", "--format", "json"))
	var observation statusResult
	if err := json.Unmarshal(stdout.Bytes(), &observation); err != nil {
		t.Fatal(err)
	}
	if exitCode(err) != 1 || len(observation.Instances) != 1 || !hasStatusCode(observation, "invalid_port") {
		t.Fatalf("status did not report invalid port: %+v, %v", observation, err)
	}
	row := observation.Instances[0]
	if row.Host == nil || *row.Host != "example.com" || row.ProjectPath == nil || *row.ProjectPath != project || row.Port != nil {
		t.Fatalf("invalid port erased identity: %+v", row)
	}
	if after := observationManifest(t, home); !reflect.DeepEqual(before, after) {
		t.Fatal("status mutated the state tree")
	}
}
