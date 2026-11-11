package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/attamusc/spin/internal/process"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use: "logs [project]", Short: "Tail the log file for a live instance",
	Long: `Show the last ~50 lines and follow live output. Ctrl-C stops tailing,
not the live process. With no project argument, resolve from the current directory.`,
	Args: cobra.MaximumNArgs(1), ValidArgsFunction: completeProjectNames, RunE: runLogs,
}

func init() { rootCmd.AddCommand(logsCmd) }

func runLogs(cmd *cobra.Command, args []string) error {
	cfg, project, err := loadProject(args)
	if err != nil {
		return err
	}
	path := process.LogFilePath(cfg.DataRoot, project.Owner, project.Repo)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "No log file found for %s\nRun 'spin up' to start the live instance first.\n", project.Name)
		return nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Tailing logs for %s (%s)\nPress Ctrl-C to stop tailing\n\n", project.Name, path)
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return tailFile(ctx, cmd.OutOrStdout(), path)
}

func tailFile(ctx context.Context, out io.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	// Bound the initial read even when the log contains very long lines.
	start := max(int64(0), info.Size()-8192)
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(file, 8192))
	if err != nil {
		return err
	}
	lines := bytes.SplitAfter(data, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if _, err := out.Write(bytes.Join(lines[max(0, len(lines)-50):], nil)); err != nil {
		return err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		current, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !os.SameFile(info, current) {
			replacement, err := os.Open(path)
			if err != nil {
				return err
			}
			file.Close()
			file, info = replacement, current
		} else if offset, err := file.Seek(0, io.SeekCurrent); err != nil {
			return err
		} else if current.Size() < offset {
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return err
			}
		}
		if _, err := io.Copy(out, file); err != nil {
			return err
		}
	}
}
