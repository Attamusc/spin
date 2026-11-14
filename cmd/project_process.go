package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
)

type liveStart struct {
	pid            int
	alreadyRunning bool
}

type liveStop struct {
	pid   int
	stale bool
}

func (p *managedProject) prepareDB(profile string) (string, error) {
	path, err := p.dbPath(profile)
	if err != nil {
		return "", fmt.Errorf("failed to resolve DB path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("could not create data directory: %w", err)
	}
	return path, nil
}

// Progress callbacks preserve pre-launch/stop messages without exposing ownership to handlers.
func (p *managedProject) startLive(beforeStart func()) (liveStart, error) {
	if err := config.ValidatePort(p.Config.Live.Port); err != nil {
		return liveStart{}, fmt.Errorf("invalid live port: %w", err)
	}
	pidFile, err := p.statePath("live.pid")
	if err != nil {
		return liveStart{}, err
	}
	release, err := process.LockProfile(p.global.DataRoot, p.Owner, p.Repo, "live")
	if err != nil {
		return liveStart{}, err
	}
	defer release()
	if _, err := process.CleanStalePIDFile(pidFile); err != nil {
		return liveStart{}, err
	}
	if pid, err := process.ReadPID(pidFile); err == nil {
		return liveStart{pid: pid, alreadyRunning: true}, nil
	} else if !os.IsNotExist(err) {
		return liveStart{}, err
	}
	beforeStart()
	dbPath, err := p.prepareDB("live")
	if err != nil {
		return liveStart{}, err
	}
	logFile, err := p.statePath("logs", "live.log")
	if err != nil {
		return liveStart{}, err
	}
	pid, err := process.StartBackground(process.BackgroundOptions{
		Command: p.Config.Live.Command, Dir: p.Dir,
		Env:     config.ResolveEnv(p.Config.Live.Env, dbPath),
		LogFile: logFile, PIDFile: pidFile, PreHook: p.Config.Live.PreHook,
	})
	if err != nil {
		return liveStart{}, fmt.Errorf("failed to start live mode: %w", err)
	}
	return liveStart{pid: pid}, nil
}

func (p *managedProject) stopLive(beforeStop func(int)) (liveStop, error) {
	pidFile, err := p.statePath("live.pid")
	if err != nil {
		return liveStop{}, err
	}
	release, err := process.LockProfile(p.global.DataRoot, p.Owner, p.Repo, "live")
	if err != nil {
		return liveStop{}, err
	}
	defer release()
	pid, err := process.ReadPID(pidFile)
	if os.IsNotExist(err) {
		return liveStop{}, nil
	}
	if err != nil {
		return liveStop{}, fmt.Errorf("could not read PID file: %w", err)
	}
	if stale, err := process.CleanStalePIDFile(pidFile); err != nil {
		return liveStop{}, err
	} else if stale {
		return liveStop{pid: pid, stale: true}, nil
	}
	beforeStop(pid)
	if err := process.StopBackground(pid); err != nil {
		return liveStop{}, fmt.Errorf("failed to stop process: %w", err)
	}
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return liveStop{}, err
	}
	return liveStop{pid: pid}, nil
}

func (p *managedProject) runDev(beforeStart func()) (int, error) {
	if err := config.ValidatePort(p.Config.Dev.Port); err != nil {
		return 1, fmt.Errorf("invalid dev port: %w", err)
	}
	release, err := process.LockProfile(p.global.DataRoot, p.Owner, p.Repo, "dev")
	if err != nil {
		return 1, err
	}
	defer release()
	dbPath, err := p.prepareDB("dev")
	if err != nil {
		return 1, err
	}
	beforeStart()
	return p.foreground(process.ForegroundOptions{
		Command: p.Config.Dev.Command, Dir: p.Dir,
		Env: config.ResolveEnv(p.Config.Dev.Env, dbPath), PreHook: p.Config.Dev.PreHook,
	})
}
