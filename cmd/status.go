package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use: "status", Short: "Observe recorded live-mode instances (read-only)", Args: cobra.NoArgs, RunE: runStatus,
}

func init() {
	statusCmd.Flags().String("format", "plain", "Output format: plain or json")
	rootCmd.AddCommand(statusCmd)
}

var errStatusIncomplete = errors.New("status observation incomplete")

func emitStatusDiagnostics(w io.Writer, r statusResult) {
	for _, d := range r.Diagnostics {
		if d.Path == nil {
			fmt.Fprintf(w, "%s: %s\n", d.Code, d.Message)
		} else {
			fmt.Fprintf(w, "%s: %q: %s\n", d.Code, *d.Path, d.Message)
		}
	}
}
func runStatus(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	if format != "plain" && format != "json" {
		return fmt.Errorf("invalid status format %q (expected plain or json)", format)
	}
	r := observeStatus()
	// Diagnostics are already emitted once; Cobra must not append usage/errors.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	emitStatusDiagnostics(cmd.ErrOrStderr(), r)
	var err error
	if format == "json" {
		err = json.NewEncoder(cmd.OutOrStdout()).Encode(r)
	} else {
		err = renderStatus(cmd.OutOrStdout(), r, false)
	}
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Could not write status output: %v\n", err)
		return err
	}
	if !r.Complete {
		return errStatusIncomplete
	}
	return nil
}
func number[T ~int | ~int64](v *T) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprint(*v)
}
func renderStatus(out io.Writer, r statusResult, liveOnly bool) error {
	rows := []statusInstance{}
	for _, row := range r.Instances {
		if !liveOnly || row.State == "live" {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		if r.Complete {
			_, err := fmt.Fprintln(out, "No live instances running.")
			return err
		}
		_, err := fmt.Fprintln(out, "Status observation incomplete.")
		return err
	}
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tMODE\tSTATE\tPORT\tPID\tUPTIME (EST.)")
	for _, row := range rows {
		uptime := "-"
		if row.UptimeSeconds != nil {
			uptime = formatDuration(time.Duration(*row.UptimeSeconds) * time.Second)
		}
		fmt.Fprintf(w, "%s/%s\t%s\t%s\t%s\t%s\t%s\n", row.Owner, row.Repo, row.Mode, row.State, number(row.Port), number(row.PID), uptime)
	}
	return w.Flush()
}

// formatDuration formats a duration as a human-readable string (e.g., "2h13m")
func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	if d < time.Minute {
		return "<1m"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
