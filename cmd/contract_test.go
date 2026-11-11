package cmd

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type contractFlag struct{ Name, Shorthand, Type, Default string }
type contractCommand struct {
	Path, Use string
	Aliases   []string
	Flags     []contractFlag
}
type cliContract struct {
	Revision string
	Commands []contractCommand
}

func commandContract() cliContract {
	// Execute normally installs these before dispatch; initialize without running a command.
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()
	result := cliContract{Revision: "d77cc8fd"}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Hidden {
			return
		}
		c.InitDefaultHelpFlag()
		aliases := append([]string{}, c.Aliases...)
		sort.Strings(aliases)
		entry := contractCommand{Path: c.CommandPath(), Use: c.Use, Aliases: aliases, Flags: []contractFlag{}}
		flags := pflag.NewFlagSet("contract", pflag.ContinueOnError)
		flags.AddFlagSet(c.InheritedFlags())
		flags.AddFlagSet(c.Flags())
		flags.VisitAll(func(f *pflag.Flag) {
			entry.Flags = append(entry.Flags, contractFlag{f.Name, f.Shorthand, f.Value.Type(), f.DefValue})
		})
		result.Commands = append(result.Commands, entry)
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(rootCmd)
	return result
}

func TestCLIContract(t *testing.T) {
	actual := commandContract()
	// Used only in a disposable original-revision source copy to produce the oracle.
	if output := os.Getenv("SPIN_BASELINE_CONTRACT_OUTPUT"); output != "" {
		data, err := json.MarshalIndent(actual, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile("testdata/cli-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected cliContract
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		got, _ := json.MarshalIndent(actual, "", "  ")
		t.Fatalf("CLI differs from original %s contract; current:\n%s", expected.Revision, got)
	}
}
