// Package refresh lists workspaces in the current org
package refresh

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func NewCmdRefresh(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"housekeeping": ""},
		Use:         "refresh",
		Short:       "Refresh the cache",
		Long:        "Refresh Brev's cache",
		Example:     `brev refresh`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := refresh(t)
			return breverrors.WrapAndTrace(err)
			// return nil
		},
	}

	return cmd
}

func refresh(t *terminal.Terminal) error {
	bar := t.NewProgressBar("Fetching orgs and workspaces", func() {})
	bar.AdvanceTo(50)

	_, _, err := brevapi.WriteCaches()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	bar.AdvanceTo(100)
	t.Vprintf(t.Green("\nCache has been refreshed\n"))

	// s := t.NewSpinner()
	// s.Start()
	// s.Suffix = "  :updating the cache" // Append text after the spinner
	
	// s.Suffix = "  :updated ✅" // Append text after the spinner
	// time.Sleep(4 * time.Second)  // Run for some time to simulate work
	// s.Stop()
	// s.Start()
	// s.Suffix = "  :updating the cache2" // Append text after the spinner
	
	// s.Suffix = "  :updated2 ✅" // Append text after the spinner
	// time.Sleep(4 * time.Second)  // Run for some time to simulate work
	// s.Stop()

	return nil
}
