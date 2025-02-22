// Package stop is for stopping Brev workspaces
package stop

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	stopLong    = "Stop a Brev machine that's in a running state"
	stopExample = "brev stop <ws_name>"
)

type StopStore interface {
	completions.CompletionStore
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	StopWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	IsWorkspace() (bool, error)
	GetCurrentWorkspaceID() (string, error)
}

func NewCmdStop(t *terminal.Terminal, loginStopStore StopStore, noLoginStopStore StopStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "stop",
		DisableFlagsInUseLine: true,
		Short:                 "Stop a workspace if it's running",
		Long:                  stopLong,
		Example:               stopExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStopStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := stopWorkspace(args[0], t, loginStopStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func stopWorkspace(workspaceName string, t *terminal.Terminal, stopStore StopStore) error {
	workspace, err := getWorkspaceFromNameOrID(workspaceName, stopStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = stopStore.StopWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Green("Workspace "+workspace.Name+" is stopping.") +
		"\nNote: this can take a few seconds. Run 'brev ls' to check status\n")

	return nil
}

func StopThisWorkspace(store StopStore, _ *terminal.Terminal) error {
	isWorkspace, err := store.IsWorkspace()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if isWorkspace {
		_ = 0
		// get current workspace
		// stopWorkspace("")
		// stop the workspace
	} else {
		return breverrors.NewValidationError("this is not a workspace -- please provide a workspace id")
	}
	return nil
}

// NOTE: this function is copy/pasted in many places. If you modify it, modify it elsewhere.
// Reasoning: there wasn't a utils file so I didn't know where to put it
//                + not sure how to pass a generic "store" object
func getWorkspaceFromNameOrID(nameOrID string, sstore StopStore) (*entity.WorkspaceWithMeta, error) {
	// Get Active Org
	org, err := sstore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, breverrors.NewValidationError("no orgs exist")
	}

	// Get Current User
	currentUser, err := sstore.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// Get Workspaces for User
	var workspace *entity.Workspace // this will be the returned workspace
	workspaces, err := sstore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{Name: nameOrID, UserID: currentUser.ID})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	switch len(workspaces) {
	case 0:
		// In this case, check workspace by ID
		wsbyid, othererr := sstore.GetWorkspace(nameOrID) // Note: workspaceName is ID in this case
		if othererr != nil {
			return nil, breverrors.NewValidationError(fmt.Sprintf("no workspaces found with name or id %s", nameOrID))
		}
		if wsbyid != nil {
			workspace = wsbyid
		} else {
			// Can this case happen?
			return nil, breverrors.NewValidationError(fmt.Sprintf("no workspaces found with name or id %s", nameOrID))
		}
	case 1:
		workspace = &workspaces[0]
	default:
		return nil, breverrors.NewValidationError(fmt.Sprintf("multiple workspaces found with name %s\n\nTry running the command by id instead of name:\n\tbrev command <id>", nameOrID))
	}

	if workspace == nil {
		return nil, breverrors.NewValidationError(fmt.Sprintf("no workspaces found with name or id %s", nameOrID))
	}

	// Get WorkspaceMetaData
	workspaceMetaData, err := sstore.GetWorkspaceMetaData(workspace.ID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &entity.WorkspaceWithMeta{WorkspaceMetaData: *workspaceMetaData, Workspace: *workspace}, nil
}
