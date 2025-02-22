package up

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/sshall"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/k8s"
	ssh "github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

type upOptions struct {
	on            *Up
	upStore       UpStore
	jetbrainsOnly bool
}

func NewCmdJetbrains(upStore UpStore, t *terminal.Terminal, jetbrainsOnly bool) *cobra.Command {
	opts := upOptions{upStore: upStore, jetbrainsOnly: jetbrainsOnly}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "jetbrains",
		DisableFlagsInUseLine: true,
		Short:                 "Run a helper proxy for required by jetbrains products",
		Long:                  "This command runs a helper proxy for jetbrains products that allows your jetbrains IDEs ssh access. It does not update if new workspaces are created or deleted so stop the process and re-run it.",
		Example:               "brev jetbrains",
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := opts.Complete(t, cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			err = opts.Validate(t)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			err = opts.RunOn(t)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func (s *upOptions) Complete(t *terminal.Terminal, _ *cobra.Command, _ []string) error {
	// spinner := t.NewSpinner()
	// spinner.Suffix = "  Setting up client"
	t.Print("Setting up client...")
	// spinner.Start()

	workspaceGroupClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(s.upStore) // to resolve
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// spinner.Suffix = "  Resolving workspaces"
	t.Print("Resolving workspaces...")
	workspaces, err := GetActiveWorkspaces(s.upStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var runningWorkspaces []entity.WorkspaceWithMeta
	for _, w := range workspaces {
		if w.Status == "RUNNING" {
			runningWorkspaces = append(runningWorkspaces, w)
		}
	}
	if len(runningWorkspaces) != len(workspaces) {
		// if above message of skipped workspaces was displayed, show how to start:
		t.Vprint(t.Yellow("\tYou can start a workspace with %s", t.Green("$ brev start <name>")))
	}

	var sshConfigurer SSHConfigurer
	if s.jetbrainsOnly {
		doesJbPathExist, err := s.upStore.DoesJetbrainsFilePathExist()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if !doesJbPathExist {
			t.Print(t.Red("\n\nYou do not have JetBrains Gateway installed."))
			t.Print("")
			t.Print(t.Yellow("Click here to install manually: "))
			t.Print("https://www.jetbrains.com/remote-development/gateway")
			t.Print("")
			return fmt.Errorf("jetbrains dne")
		}
		jbConfig, err := ssh.NewJetBrainsGatewayConfig(s.upStore)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		// copy values so we aren't modifying eachother
		reader := jbConfig
		sshConfigWriter := jbConfig
		sshConfigurer = ssh.NewSSHConfigurer(runningWorkspaces, reader, []ssh.Writer{sshConfigWriter}, s.upStore, workspaceGroupClientMapper.GetPrivateKey())
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

	} else {
		sshConfig, err := ssh.NewSSHConfig(s.upStore)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		// copy values so we aren't modifying eachother
		reader := sshConfig
		sshConfigWriter := sshConfig
		sshConfigurer = ssh.NewSSHConfigurer(runningWorkspaces, reader, []ssh.Writer{sshConfigWriter}, s.upStore, workspaceGroupClientMapper.GetPrivateKey())
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	s.on = NewUp(runningWorkspaces, sshConfigurer, workspaceGroupClientMapper)
	// spinner.Stop()
	return nil
}

type UpStore interface {
	ssh.SSHStore
	ssh.SSHConfigurerStore
	ssh.JetBrainsGatewayConfigStore
	k8s.K8sStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	GetCurrentUser() (*entity.User, error)
	DoesJetbrainsFilePathExist() (bool, error)
}

func (s upOptions) Validate(_ *terminal.Terminal) error {
	return nil
}

func (s upOptions) RunOn(_ *terminal.Terminal) error {
	fmt.Println("Running up...")
	return s.on.Run()
}

type SSHConfigurer interface {
	Sync() error
	sshall.SSHResolver
}

type Up struct {
	sshConfigurer              SSHConfigurer
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
	workspaces                 []entity.WorkspaceWithMeta
}

func NewUp(workspaces []entity.WorkspaceWithMeta, sshConfigurer SSHConfigurer, workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper) *Up {
	return &Up{
		workspaces:                 workspaces,
		sshConfigurer:              sshConfigurer,
		workspaceGroupClientMapper: workspaceGroupClientMapper,
	}
}

func (o Up) Run() error {
	err := o.sshConfigurer.Sync()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	sshall := sshall.NewSSHAll(o.workspaces, o.workspaceGroupClientMapper, o.sshConfigurer)
	err = sshall.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func GetActiveWorkspaces(upStore UpStore) ([]entity.WorkspaceWithMeta, error) {
	// fmt.Println("Resolving workspaces...")

	org, err := upStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, fmt.Errorf("no orgs exist")
	}

	user, err := upStore.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspaces, err := upStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{
		UserID: user.ID,
	})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var workspacesWithMeta []entity.WorkspaceWithMeta
	for _, w := range workspaces {
		wmeta, err := upStore.GetWorkspaceMetaData(w.ID)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}

		workspaceWithMeta := entity.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}
		workspacesWithMeta = append(workspacesWithMeta, workspaceWithMeta)
	}
	return workspacesWithMeta, nil
}
