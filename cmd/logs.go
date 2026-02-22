package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [project]",
	Short: "Tail the log file for a live instance",
	Long: `Tail the log file for a project's live mode background process.
Shows the last ~50 lines, then follows new output.
Ctrl-C exits the tail without affecting the running process.

If no project argument is given, the project is resolved from the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	var project *resolve.ResolvedProject
	if len(args) > 0 {
		return fmt.Errorf("project argument not yet supported, run from within a project directory")
	}

	project, err = resolve.FromCWD(globalCfg.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve project: %w", err)
	}

	logPath := process.LogFilePath(globalCfg.DataRoot, project.Owner, project.Repo)

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No log file found for %s\n", project.Name)
		fmt.Fprintf(os.Stderr, "Run 'spin up' to start the live instance first.\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Tailing logs for %s (%s)\n", project.Name, logPath)
	fmt.Fprintf(os.Stderr, "Press Ctrl-C to stop tailing\n\n")

	return tailFile(logPath)
}

// tailFile tails a file, printing the last ~50 lines then following new content.
func tailFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}
	defer f.Close()

	// Print last ~50 lines
	printLastLines(f, 50)

	// Get current position for following
	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	// Set up interrupt handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Poll for new content
	buf := make([]byte, 4096)
	for {
		select {
		case <-sigChan:
			fmt.Println()
			return nil
		default:
		}

		n, err := f.ReadAt(buf, offset)
		if n > 0 {
			os.Stdout.Write(buf[:n])
			offset += int64(n)
		}
		if err != nil && err != io.EOF {
			return err
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// printLastLines prints the last n lines from the current position in the file.
func printLastLines(f *os.File, n int) {
	// Get file size
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return
	}

	// Read file from the end to find the last n newlines
	size := info.Size()
	bufSize := int64(8192)
	if bufSize > size {
		bufSize = size
	}

	// Read the tail of the file
	buf := make([]byte, bufSize)
	readAt := size - bufSize
	_, err = f.ReadAt(buf, readAt)
	if err != nil && err != io.EOF {
		return
	}

	// Count newlines from the end
	lines := 0
	pos := len(buf) - 1

	// Skip trailing newline
	if pos >= 0 && buf[pos] == '\n' {
		pos--
	}

	for pos >= 0 {
		if buf[pos] == '\n' {
			lines++
			if lines >= n {
				pos++
				break
			}
		}
		pos--
	}
	if pos < 0 {
		pos = 0
	}

	os.Stdout.Write(buf[pos:])

	// Seek to end for following
	f.Seek(0, io.SeekEnd)
}
