package vcs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFakeTools(t *testing.T) {
	for _, tool := range []string{"jj", "git"} {
		t.Run(tool, func(t *testing.T) {
			tools, repo := t.TempDir(), t.TempDir()
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("PATH", tools)
			t.Setenv("CALLS", log)
			for _, name := range []string{"jj", "git"} {
				if tool == "git" && name == "jj" {
					continue
				}
				if err := os.WriteFile(filepath.Join(tools, name), []byte("#!/bin/sh\nprintf '%s:%s\\n' \"${0##*/}\" \"$*\" >> \"$CALLS\"\n"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(filepath.Join(repo, "."+tool), 0755); err != nil {
				t.Fatal(err)
			}
			for _, err := range []error{Clone("fake-url", "target"), Init(repo), Fetch(repo)} {
				if err != nil {
					t.Fatal(err)
				}
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(calls), tool+":") != 3 {
				t.Fatalf("wrong routing: %s", calls)
			}
			if tool == "jj" && !strings.Contains(string(calls), "git clone --colocate fake-url target") {
				t.Fatalf("wrong clone args: %s", calls)
			}
			if tool == "git" && !strings.Contains(string(calls), "fetch --all") {
				t.Fatalf("wrong fetch args: %s", calls)
			}
		})
	}
}

func TestFetchContextCancellation(t *testing.T) {
	tools, repo := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "git"), []byte("#!/bin/sh\nwhile :; do :; done\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := FetchContext(ctx, repo); err == nil {
		t.Fatal("expected cancellation")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("cancellation delayed")
	}
}
