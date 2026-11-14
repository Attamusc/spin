package process

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
)

func TestTerminateGroupReapsExitedChild(t *testing.T) {
	child := exec.Command("sleep", "30")
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			child.Process.Kill()
			child.Wait()
		}
	})
	probes, denied := 0, 0
	err := terminateGroup(child.Process.Pid, func(pid int, sig syscall.Signal) error {
		err := syscall.Kill(pid, sig)
		if sig == 0 {
			probes++
			if errors.Is(err, syscall.EPERM) {
				denied++
			}
			// Hold the exited child unreaped to exercise Darwin's zombie-only
			// group. Kernels that still report the group live use the third poll.
			if !reaped && (errors.Is(err, syscall.EPERM) || err == nil && probes >= 3) {
				child.Wait()
				reaped = true
			}
		}
		return err
	}, sleepMs)
	if err != nil {
		t.Fatal(err)
	}
	if !reaped {
		child.Wait()
		reaped = true
	}
	if err := syscall.Kill(-child.Process.Pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("reaped group did not disappear: %v", err)
	}
	t.Logf("post-TERM probes=%d permission-denied=%d; reaped group is gone", probes, denied)
}

func TestTerminateGroupPermissionAndEscalation(t *testing.T) {
	lastDenied := make([]error, 50)
	lastDenied[49] = syscall.EPERM
	for _, tc := range []struct {
		name                                string
		termErr, probeErr, killErr, wantErr error
		sequence                            []error
		wantProbes, wantPauses, wantKills   int
	}{
		{name: "TERM denied", termErr: syscall.EPERM, wantErr: syscall.EPERM},
		{name: "gone before TERM", termErr: syscall.ESRCH},
		{name: "gone after TERM", probeErr: syscall.ESRCH, wantProbes: 1},
		{name: "persistent denial never escalates", probeErr: syscall.EPERM, wantErr: syscall.EPERM, wantProbes: 50, wantPauses: 50},
		{name: "last probe denied never escalates", sequence: lastDenied, wantErr: syscall.EPERM, wantProbes: 50, wantPauses: 50},
		{name: "other probe error fails closed", probeErr: syscall.EIO, wantErr: syscall.EIO, wantProbes: 1},
		{name: "transient denial then live then gone", sequence: []error{syscall.EPERM, nil}, probeErr: syscall.ESRCH, wantProbes: 3, wantPauses: 2},
		{name: "live group escalates", wantProbes: 50, wantPauses: 50, wantKills: 1},
		{name: "gone before KILL", killErr: syscall.ESRCH, wantProbes: 50, wantPauses: 50, wantKills: 1},
		{name: "KILL denied", killErr: syscall.EPERM, wantErr: syscall.EPERM, wantProbes: 50, wantPauses: 50, wantKills: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			terms, probes, pauses, kills := 0, 0, 0, 0
			err := terminateGroup(1234, func(pid int, sig syscall.Signal) error {
				if pid != -1234 {
					t.Fatalf("signal targeted %d, want process group -1234", pid)
				}
				switch sig {
				case syscall.SIGTERM:
					terms++
					return tc.termErr
				case 0:
					index := probes
					probes++
					if index < len(tc.sequence) {
						return tc.sequence[index]
					}
					return tc.probeErr
				case syscall.SIGKILL:
					kills++
					return tc.killErr
				default:
					t.Fatalf("unexpected signal: %v", sig)
					return nil
				}
			}, func(ms int) {
				if ms != 100 {
					t.Fatalf("unexpected poll delay: %d", ms)
				}
				pauses++
			})
			if !errors.Is(err, tc.wantErr) || terms != 1 || probes != tc.wantProbes || pauses != tc.wantPauses || kills != tc.wantKills {
				t.Fatalf("terms=%d probes=%d pauses=%d kills=%d err=%v; want probes=%d pauses=%d kills=%d err=%v", terms, probes, pauses, kills, err, tc.wantProbes, tc.wantPauses, tc.wantKills, tc.wantErr)
			}
		})
	}
}

func TestTerminateGroupWaitsForReapedGroup(t *testing.T) {
	probes, pauses := 0, 0
	err := terminateGroup(1234, func(pid int, sig syscall.Signal) error {
		if pid != -1234 {
			t.Fatalf("signal targeted %d, want process group -1234", pid)
		}
		switch sig {
		case syscall.SIGTERM:
			return nil
		case 0:
			probes++
			if probes == 1 {
				return syscall.EPERM
			}
			return syscall.ESRCH
		default:
			t.Fatalf("unexpected escalation: %v", sig)
			return nil
		}
	}, func(ms int) {
		if ms != 100 {
			t.Fatalf("unexpected poll delay: %d", ms)
		}
		pauses++
	})
	if err != nil || probes != 2 || pauses != 1 {
		t.Fatalf("shutdown result: probes=%d pauses=%d err=%v", probes, pauses, err)
	}
}
