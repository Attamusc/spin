package cmd

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestOpenerForOS(t *testing.T) {
	for _, tc := range []struct {
		goos string
		want string
	}{
		{goos: "darwin", want: "open"},
		{goos: "linux", want: "xdg-open"},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			got, err := openerForOS(tc.goos)
			if err != nil {
				t.Fatalf("openerForOS(%q) error: %v", tc.goos, err)
			}
			if got != tc.want {
				t.Fatalf("openerForOS(%q) = %q, want %q", tc.goos, got, tc.want)
			}
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		_, err := openerForOS("plan9")
		if err == nil {
			t.Fatal("openerForOS(\"plan9\") returned nil error")
		}
		if !strings.Contains(err.Error(), `unsupported operating system "plan9"`) {
			t.Fatalf("openerForOS(\"plan9\") error = %q", err)
		}
	})
}

func TestOpenURLRunsPlatformOpener(t *testing.T) {
	var gotName string
	var gotArgs []string
	run := func(name string, args ...string) error {
		gotName = name
		gotArgs = args
		return nil
	}

	if err := openURL("linux", "http://localhost:3000", run); err != nil {
		t.Fatalf("openURL() error: %v", err)
	}
	if gotName != "xdg-open" {
		t.Fatalf("runner command = %q, want %q", gotName, "xdg-open")
	}
	if want := []string{"http://localhost:3000"}; !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("runner args = %q, want %q", gotArgs, want)
	}
}

func TestOpenURLReportsOpenerFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		runErr error
	}{
		{name: "missing", runErr: errors.New("executable file not found")},
		{name: "failed", runErr: errors.New("exit status 1")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := openURL("linux", "http://localhost:3000", func(string, ...string) error {
				return tc.runErr
			})
			if !errors.Is(err, tc.runErr) {
				t.Fatalf("openURL() error = %v, want wrapped %v", err, tc.runErr)
			}
			if !strings.Contains(err.Error(), "failed to open browser with xdg-open") {
				t.Fatalf("openURL() error = %q", err)
			}
		})
	}
}
