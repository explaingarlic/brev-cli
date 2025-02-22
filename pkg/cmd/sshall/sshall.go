package sshall

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"sync"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/runtime"
)

type (
	connectionMap map[entity.WorkspaceWithMeta]chan struct{}
	retrymap      map[entity.WorkspaceWithMeta]int
)

type SSHAll struct {
	workspaces                 []entity.WorkspaceWithMeta
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
	sshResolver                SSHResolver
	workspaceConnections       connectionMap
	workspaceConnectionsMutex  *sync.RWMutex
	retries                    retrymap
}

type SSHResolver interface {
	GetConfiguredWorkspacePort(entity.WorkspaceLocalID) (string, error)
	GetPrivateKeyPath() (string, error)
}

func NewSSHAll(
	workspaces []entity.WorkspaceWithMeta,
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper,
	sshResolver SSHResolver,
) *SSHAll {
	return &SSHAll{
		workspaces:                 workspaces,
		workspaceGroupClientMapper: workspaceGroupClientMapper,
		sshResolver:                sshResolver,
		workspaceConnections:       make(connectionMap),
		workspaceConnectionsMutex:  &sync.RWMutex{},
		retries:                    make(retrymap),
	}
}

func (s SSHAll) workspaceSSHConnectionHealthCheck(w entity.WorkspaceWithMeta) (bool, error) {
	var hostKey ssh.PublicKey
	// A public key may be used to authenticate against the remote
	// 	var hostKey ssh.PublicKey
	// A public key may be used to authenticate against the remote
	// server by using an unencrypted PEM-encoded private key file.
	//
	// If you have an encrypted private key, the crypto/x509 package
	// can be used to decrypt it.
	privateKeyPath, err := s.sshResolver.GetPrivateKeyPath()
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	key, err := ioutil.ReadFile(privateKeyPath) //nolint:gosec // not including file via variable
	if err != nil {
		return false, breverrors.WrapAndTrace(err, "unable to read private key: %v")
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return false, breverrors.WrapAndTrace(err, "unable to parse private key: %v")
	}

	config := &ssh.ClientConfig{
		User: "brev",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.FixedHostKey(hostKey),
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", w.DNS, config)
	if err != nil {
		return false, breverrors.WrapAndTrace(err, "unable to connect: %v")
	}
	err = client.Close()
	if err != nil {
		return true, breverrors.WrapAndTrace(err)
	}
	return true, nil
}

// NOTE: not sure if passing in all the workspaces to check for name is useful here, like in entity.go: GetLocalIdentifier
func (s SSHAll) runPortForwardWorkspace(workspace entity.WorkspaceWithMeta, _ []entity.WorkspaceWithMeta) {
	err := s.portforwardWorkspace(workspace)
	if err != nil {
		// todo have verbose version with trace
		fmt.Printf("%v [workspace=%s]\n", errors.Cause(err), workspace.DNS)
	}
}

func (s SSHAll) Run() error {
	errorQueue := make(chan error, 1)

	// set up error handling for the ssh connections, very much brute force
	// since kubectl was not intended to be used a library like this
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		errorQueue <- err
	})

	go func() {
		for {
			<-errorQueue
			fmt.Println("resetting unhealthy connections")
			for _, w := range s.workspaces {
				isHealthy, _ := s.workspaceSSHConnectionHealthCheck(w)
				if !isHealthy {
					fmt.Printf("resetting [w=%s]\n", w.DNS)
					TryClose(s.workspaceConnections[w])
					if s.retries[w] > 0 {
						s.retries[w]--
						s.runPortForwardWorkspace(w, s.workspaces)
					}
				}
			}
		}
	}()

	if len(s.workspaces) == 0 {
		fmt.Println("No workspaces in org")
		return nil
	}

	fmt.Println(`Please add the following to your ssh config to
avoid false positives for user host key checking.

Host *
  NoHostAuthenticationForLocalhost yes

	`)

	fmt.Println()
	for _, w := range s.workspaces {
		fmt.Printf("ssh %s\n", w.GetLocalIdentifier())
		s.retries[w] = 3 // TODO magic number
		s.runPortForwardWorkspace(w, s.workspaces)
	}
	fmt.Println()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)
	<-signals

	return nil
}

func TryClose(toClose chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered panic:", r)
		}
	}()
	close(toClose)
}

// NADER IS SO FUCKING SORRY FOR DOING THIS TWICE BUT I HAVE NO CLUE WHERE THIS HELPER FUNCTION SHOULD GO SO ITS COPY/PASTED ELSEWHERE
// IF YOU MODIFY IT MODIFY IT EVERYWHERE OR PLEASE PUT IT IN ITS PROPER PLACE. thank you you're the best <3
func WorkspacesFromWorkspaceWithMeta(wwm []entity.WorkspaceWithMeta) []entity.Workspace {
	var workspaces []entity.Workspace

	for _, v := range wwm {
		workspaces = append(workspaces, entity.Workspace{
			ID:                v.ID,
			Name:              v.Name,
			WorkspaceGroupID:  v.WorkspaceGroupID,
			OrganizationID:    v.OrganizationID,
			WorkspaceClassID:  v.WorkspaceClassID,
			CreatedByUserID:   v.CreatedByUserID,
			DNS:               v.DNS,
			Status:            v.Status,
			Password:          v.Password,
			GitRepo:           v.GitRepo,
			Version:           v.Version,
			WorkspaceTemplate: v.WorkspaceTemplate,
		})
	}

	return workspaces
}

func (s SSHAll) portforwardWorkspace(workspace entity.WorkspaceWithMeta) error {
	port, err := s.sshResolver.GetConfiguredWorkspacePort(workspace.Workspace.GetLocalIdentifier())
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if port == "" {
		return fmt.Errorf("port not found")
	}
	portMapping := makeSSHPortMapping(port)
	err = s.portforwardWorkspaceAtPort(workspace, portMapping)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (s *SSHAll) portforwardWorkspaceAtPort(workspace entity.WorkspaceWithMeta, portMapping string) error {
	dpf := portforward.NewDefaultPortForwarder()
	pf := portforward.NewPortForwardOptions(
		s.workspaceGroupClientMapper,
		dpf,
	)

	s.workspaceConnectionsMutex.Lock()
	s.workspaceConnections[workspace] = pf.StopChannel
	s.workspaceConnectionsMutex.Unlock()

	_, err := pf.WithWorkspace(workspace)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	pf.WithPort(portMapping)

	go func() {
		err = pf.RunPortforward()
		if err != nil {
			log.Println(err)
		}
	}()

	return nil
}

type (
	RandomSSHResolver struct {
		WorkspaceResolver WorkspaceResolver
	}
	WorkspaceResolver interface {
		GetMyWorkspaces(orgID string) ([]entity.Workspace, error)
		GetWorkspaceMetaData(wsID string) (*entity.WorkspaceMetaData, error)
		GetActiveOrganizationOrDefault() (*entity.Organization, error)
	}
)

func NewRandomPortSSHResolver(workspaceResolver WorkspaceResolver) *RandomSSHResolver {
	return &RandomSSHResolver{
		WorkspaceResolver: workspaceResolver,
	}
}

func (r RandomSSHResolver) GetWorkspaces() ([]entity.WorkspaceWithMeta, error) {
	activeOrg, err := r.WorkspaceResolver.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	wss, err := r.WorkspaceResolver.GetMyWorkspaces(activeOrg.ID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var workspacesWithMeta []entity.WorkspaceWithMeta
	for _, w := range wss {
		wmeta, err := r.WorkspaceResolver.GetWorkspaceMetaData(w.ID)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}

		workspaceWithMeta := entity.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}
		workspacesWithMeta = append(workspacesWithMeta, workspaceWithMeta)
	}

	return workspacesWithMeta, nil
}

func (r RandomSSHResolver) GetConfiguredWorkspacePort(_ entity.Workspace) (string, error) {
	minPort := 1024
	maxPort := 65535
	port := rand.Intn(maxPort-minPort) + minPort // #nosec no need to by cryptographically secure
	return strconv.Itoa(port), nil
}

func makeSSHPortMapping(localPort string) string {
	return fmt.Sprintf("%s:22", localPort)
}
