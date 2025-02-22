package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

type WorkspaceTestClient struct {
	SetupParams     *store.SetupParamsV0
	ContainerParams []ContainerParams
	TestWorkspaces  []Workspace
}

type ContainerParams struct {
	Name  string
	Image string
	Ports []string
}

type BrevBinaryPath struct {
	BinaryPath string
}

func (b BrevBinaryPath) ApplyWorkspaceTestClientOption(allOptions *AllWorkspaceTestClientOption) {
	allOptions.BrevBinaryPath = b.BinaryPath
}

type WorkspaceTestClientOption interface {
	ApplyWorkspaceTestClientOption(allOptions *AllWorkspaceTestClientOption)
}

type AllWorkspaceTestClientOption struct {
	BrevBinaryPath string
	TestNamePrefix string
}

func NewWorkspaceTestClientOptions(options []WorkspaceTestClientOption) AllWorkspaceTestClientOption {
	allOptions := AllWorkspaceTestClientOption{}
	for _, o := range options {
		o.ApplyWorkspaceTestClientOption(&allOptions)
	}
	return allOptions
}

type TestNamePrefix struct {
	Name string
}

func (t TestNamePrefix) ApplyWorkspaceTestClientOption(allOptions *AllWorkspaceTestClientOption) {
	allOptions.TestNamePrefix = t.Name
}

func NewWorkspaceTestClient(setupParams *store.SetupParamsV0, containerParams []ContainerParams, options ...WorkspaceTestClientOption) *WorkspaceTestClient {
	allOptions := NewWorkspaceTestClientOptions(options)

	if allOptions.TestNamePrefix == "" {
		pc, _, _, ok := runtime.Caller(1)
		if !ok {
			panic("not ok")
		}
		details := runtime.FuncForPC(pc)
		allOptions.TestNamePrefix = strings.Split(details.Name(), ".")[2]
	}

	workspaces := []Workspace{}
	for _, p := range containerParams {
		containerName := fmt.Sprintf("%s-%s", allOptions.TestNamePrefix, p.Name)
		// [a-zA-Z0-9][a-zA-Z0-9_.-]
		workspace := NewTestWorkspace(allOptions.BrevBinaryPath, containerName, p.Image, p.Ports, setupParams)
		_ = workspace.Done()
		workspace.ShowOut = true
		workspaces = append(workspaces, workspace)
	}

	return &WorkspaceTestClient{
		SetupParams:     setupParams,
		ContainerParams: containerParams,
		TestWorkspaces:  workspaces,
	}
}

func (w WorkspaceTestClient) Done() error {
	var allError error
	for _, w := range w.TestWorkspaces {
		err := w.Done()
		if err != nil {
			allError = multierror.Append(allError, err)
		}
	}
	if allError != nil {
		return breverrors.WrapAndTrace(allError)
	}
	return nil
}

type workspaceTest func(workspace Workspace, err error)

func (w WorkspaceTestClient) Test(test workspaceTest) error {
	for _, w := range w.TestWorkspaces {
		err := w.Setup()
		test(w, err)
	}
	return nil
}

type Workspace interface {
	Setup() error
	Done() error
	Reset() error
	Exec(arg ...string) ([]byte, error) // always returns []byte even if error since stdout/err is still useful
	Copy(src string, dest string) error
	UpdateParams(*store.SetupParamsV0)
}

type ExecResult interface {
	CombinedOutput() ([]byte, error)
}

type TestWorkspace struct {
	SetupParams    *store.SetupParamsV0
	ContainerName  string
	Image          string
	Ports          []string
	TestBrevBinary string // path to brev binary that should be tested
	ShowOut        bool
}

var _ Workspace = &TestWorkspace{}

func NewTestWorkspace(testBrevBinaryPath string, containerName string, image string, ports []string, setupParams *store.SetupParamsV0) *TestWorkspace {
	return &TestWorkspace{SetupParams: setupParams, ContainerName: containerName, Ports: ports, Image: image, TestBrevBinary: testBrevBinaryPath}
}

func (w *TestWorkspace) UpdateParams(params *store.SetupParamsV0) {
	w.SetupParams = params
}

func sendToOut(c *exec.Cmd) {
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
}

func (w TestWorkspace) getWorkspaceVolumeName() string {
	return fmt.Sprintf("%s-workspace", w.ContainerName)
}

func (w TestWorkspace) Setup() error {
	ports := []string{}
	for _, p := range w.Ports {
		ports = append(ports, "-p", p)
	}

	dockerRunArgs := append([]string{
		"-d",
		"--privileged=true",
		fmt.Sprintf("--name=%s", w.ContainerName),
		// "-v", fmt.Sprintf("%s:/home/brev/workspace", w.getWorkspaceVolumeName()),
		"-v", fmt.Sprintf("%s:/home/brev", w.getWorkspaceVolumeName()),
		"--rm", "-it",
	}, ports...)

	dockerRunArgs = append(append([]string{"run"}, dockerRunArgs...), []string{w.Image, "bash"}...)

	cmdR := exec.Command("docker", dockerRunArgs...) //nolint:gosec // for tests
	if w.ShowOut {
		sendToOut(cmdR)
	}
	err := cmdR.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = w.Exec("mkdir", "-p", "/etc/meta")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	tmpSetupFile := fmt.Sprintf("/tmp/%s_setup_v0.json", w.ContainerName)
	err = files.OverwriteJSON(files.AppFs, tmpSetupFile, w.SetupParams)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	setupPath := fmt.Sprintf("%s:/etc/meta/setup_v0.json", w.ContainerName)
	err = w.Copy(tmpSetupFile, setupPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	binPath := fmt.Sprintf("%s:/usr/local/bin/brev", w.ContainerName)
	err = w.Copy(w.TestBrevBinary, binPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = w.Exec("/usr/local/bin/brev", "setupworkspace", "--force-enable")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Exec(arg ...string) ([]byte, error) {
	arg = append([]string{"exec", w.ContainerName}, arg...)
	cmdM := exec.Command("docker", arg...)
	out, err := cmdM.CombinedOutput()
	if w.ShowOut {
		fmt.Print(string(out))
	}
	if err != nil {
		return out, breverrors.WrapAndTrace(err)
	}
	return out, nil
}

func (w TestWorkspace) Copy(src string, dest string) error {
	cmdC := exec.Command("docker", "cp", src, dest)
	if w.ShowOut {
		sendToOut(cmdC)
	}
	err := cmdC.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Reset() error {
	err := w.KillContainer()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = w.Setup()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) KillContainer() error {
	cmd := exec.Command("docker", "kill", w.ContainerName) //nolint:gosec // for tests
	if w.ShowOut {
		sendToOut(cmd)
	}
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) RmContainer() error {
	cmd := exec.Command("docker", "rm", w.ContainerName) //nolint:gosec // for tests
	if w.ShowOut {
		sendToOut(cmd)
	}
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) RmVolume() error {
	cmd := exec.Command("docker", "volume", "rm", w.getWorkspaceVolumeName()) //nolint:gosec // for tests
	if w.ShowOut {
		sendToOut(cmd)
	}
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Done() error {
	err := w.KillContainer()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	_ = w.RmContainer()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = w.RmVolume()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func NewTestSetupParams(keyPair *store.KeyPair) *store.SetupParamsV0 {
	return &store.SetupParamsV0{
		WorkspaceHost:                    "name-rand-org.x.y",
		WorkspacePort:                    22778,
		WorkspaceBaseRepo:                "github.com:brevdev/test-repo-user-config.git",
		WorkspaceProjectRepo:             "github.com:brevdev/test-repo-dotbrev.git",
		WorkspaceApplicationStartScripts: []string{},
		WorkspaceUsername:                "brevtest",
		WorkspaceEmail:                   "test+3333@gmail.com",
		WorkspacePassword:                "12345",
		WorkspaceKeyPair:                 keyPair,
		SetupScript:                      nil,
		DisableSetup:                     true,
	}
}

func GetTestKeys() (*store.KeyPair, error) {
	kp := store.KeyPair{}
	err := files.ReadJSON(files.AppFs, "/home/brev/workspace/brev-cli/assets/test_keypair.json", &kp) // TODO relative path
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &kp, nil
}

func GetUnauthedTestKeys() (*store.KeyPair, error) {
	kp := store.KeyPair{}
	err := files.ReadJSON(files.AppFs, "/home/brev/workspace/brev-cli/assets/test_noauth_keypair.json", &kp) // TODO relative path
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &kp, nil
}

func AssertWorkspaceSetup(t *testing.T, w Workspace, password string, host string) {
	t.Helper()
	AssertUser(t, w, "root")
	AssertCwd(t, w, "/home/brev/workspace")

	time.Sleep(5000 * time.Millisecond) // sometimes localhost:2278 returs bad error

	AssertInternalCurlOuputContains(t, w, "localhost:22778", "Found. Redirecting to ./login")
	AssertInternalCurlOuputContains(t, w, "localhost:22779/proxy", "Bad Request")
	AssertFileContainsString(t, w, "/home/brev/.config/code-server/config.yaml", password)
	AssertFileContainsString(t, w, "/home/brev/.config/code-server/config.yaml", host)
	AssertInternalSSHServerRunning(t, w, "/home/brev/.ssh/id_rsa", "brev", "ls")
	AssertDockerRunning(t, w)
}

func AssertValidBrevBaseRepoSetup(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertPathExists(t, w, repoPath)
	AssertPathExists(t, w, fmt.Sprintf("%s/.git", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/setup.sh", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/logs", repoPath))
}

func AssertValidUserBrevSetup(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertValidBrevBaseRepoSetup(t, w, repoPath)
	AssertPathNotExist(t, w, fmt.Sprintf("%s/.brev/ports.yaml", repoPath))
}

func AssertTestUserRepoSetupRan(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", repoPath), "user setup script ran")
	AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", repoPath), "user: brev")
	AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", repoPath), fmt.Sprintf("pwd: %s", filepath.Join("/home/brev/workspace", repoPath)))
}

func AssertValidBrevProjRepo(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertValidBrevBaseRepoSetup(t, w, repoPath)

	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/.gitignore", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/ports.yaml", repoPath))
}

func AssertTestRepoSetupRan(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", repoPath), "repo setup script ran")
	AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", repoPath), "user: brev")
	AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", repoPath), fmt.Sprintf("pwd: %s", filepath.Join("/home/brev/workspace", repoPath)))
}

func AssertCwd(t *testing.T, w Workspace, expectedCwd string) {
	t.Helper()
	out, err := w.Exec("pwd")
	assert.Nil(t, err)
	assert.Equal(t, expectedCwd, strings.TrimSpace(string(out)))
}

func AssertUser(t *testing.T, w Workspace, expectedUser string) {
	t.Helper()
	out, err := w.Exec("whoami")
	assert.Nil(t, err)
	assert.Equal(t, expectedUser, strings.TrimSpace(string(out)))
}

func AssertInternalCurlOuputContains(t *testing.T, w Workspace, url string, contains string) {
	t.Helper()
	out, err := w.Exec("curl", "-s", url)
	assert.Nil(t, err)
	o := string(out)
	assert.Contains(t, o, contains)
}

// func AssertCodeServerPasswordWorks(t *testing.T) {
// }

func AssertInternalSSHServerRunning(t *testing.T, w Workspace, privKeyPath string, user string, healthCheck string) {
	t.Helper()

	if !AssertPathExists(t, w, privKeyPath) {
		return
	}

	_, err := w.Exec("ssh", "-i", privKeyPath, "-o", "StrictHostKeyChecking no", fmt.Sprintf("%s@localhost", user), healthCheck)
	assert.Nil(t, err, "can connect with private key")

	_, err = w.Exec("ssh", "-o", "StrictHostKeyChecking no", fmt.Sprintf("%s@localhost", user), healthCheck)
	assert.Error(t, err, "can not connect without private key")
}

// func AssertInternalSSHServerNoPass(t *testing.T, w Workspace, user string, pass string, healthCheck string) {
// 	t.Helper()
// 	// TODO assert can't connect with password
// 	// TODO how to pass password in one line - use sshpass?
// }

func AssertPathExists(t *testing.T, workspace Workspace, path string) bool {
	t.Helper()
	_, err := workspace.Exec("ls", path)
	return assert.Nil(t, err)
}

func AssertPathNotExist(t *testing.T, workspace Workspace, path string) bool {
	t.Helper()
	_, err := workspace.Exec("ls", path)
	return assert.NotNil(t, err)
}

func AssertPathDoesNotExist(t *testing.T, workspace Workspace, path string) bool {
	t.Helper()
	_, err := workspace.Exec("ls", path)
	return assert.Error(t, err)
}

func AssertFileContainsString(t *testing.T, w Workspace, filePath string, contains string) bool {
	t.Helper()

	res, err := w.Exec("grep", contains, filePath)
	assert.Contains(t, string(res), contains)
	return assert.Nil(t, err)
}

func AssertFileNotContainsString(t *testing.T, w Workspace, filePath string, contains string) bool {
	t.Helper()

	_, err := w.Exec("grep", contains, filePath)
	return assert.Error(t, err)
}

func AssertDockerRunning(t *testing.T, w Workspace) bool {
	t.Helper()

	_, err := w.Exec("docker", "run", "hello-world")
	return assert.Nil(t, err)
}

func AssertRepoHasNumFiles(t *testing.T, w Workspace, filePath string, num int) {
	t.Helper()
	out, err := w.Exec("ls", "-a", filePath)
	assert.Nil(t, err)
	assert.Len(t, strings.Fields(string(out)), num)
}

func UpdateFile(w Workspace, filePath string, content string) error {
	_, err := w.Exec("sh", "-c", fmt.Sprintf(`echo '%s' > %s`, content, filePath))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
