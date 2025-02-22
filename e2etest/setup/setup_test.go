package setup

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/stretchr/testify/assert"
)

func init() {
	fmt.Println("building binary")
	cmd := exec.Command("/usr/bin/make")
	cmd.Dir = "/home/brev/workspace/brev-cli" // TODO relative path
	_, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
}

func NewStdWorkspaceTestClient(setupParams *store.SetupParamsV0, containerParams []ContainerParams, options ...WorkspaceTestClientOption) *WorkspaceTestClient {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("not ok")
	}
	details := runtime.FuncForPC(pc)
	testNamePrefix := strings.Split(details.Name(), ".")[2]

	return NewWorkspaceTestClient(setupParams, containerParams, append([]WorkspaceTestClientOption{
		BrevBinaryPath{BinaryPath: "/home/brev/workspace/brev-cli/brev"}, // TODO relativ path
		TestNamePrefix{Name: testNamePrefix},
	}, options...)...)
}

var SupportedContainers = []ContainerParams{
	{
		Name:  "brevdev-ubuntu-proxy-0.3.17",
		Image: "brevdev/ubuntu-proxy:0.3.17",
		Ports: []string{},
	},
}

func Test_UserBrevProjectBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
	})

	assert.Nil(t, err)
}

func Test_NoProjectBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = ""

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertValidBrevProjRepo(t, w, "name")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertValidBrevProjRepo(t, w, "name")
	})
	assert.Nil(t, err)
}

func Test_NoUserBrevNoProj(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceBaseRepo = ""
	params.WorkspaceProjectRepo = ""

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertPathDoesNotExist(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "name")
	})
	assert.Nil(t, err)
}

func Test_NoUserBrevProj(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceBaseRepo = ""

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertPathDoesNotExist(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertPathDoesNotExist(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
	})
	assert.Nil(t, err)
}

func Test_ProjectRepoNoBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = "github.com:brevdev/test-repo-no-dotbrev.git"

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")
	})
	assert.Nil(t, err)
}

const ProvidedSetupScriptMsg = "provided setup script ran"

func Test_ProvidedSetupRanNoProj(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = ""
	setupScript := fmt.Sprintf("echo %s ", ProvidedSetupScriptMsg)
	base64SetupScript := base64.StdEncoding.EncodeToString([]byte(setupScript))
	params.SetupScript = &base64SetupScript

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "name")
		AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "name"), ProvidedSetupScriptMsg)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "name")
		AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "name"), ProvidedSetupScriptMsg)
	})
	assert.Nil(t, err)
}

func Test_ProvidedSetupRanProjNoBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = "github.com:brevdev/test-repo-no-dotbrev.git"
	setupScript := fmt.Sprintf("echo %s ", ProvidedSetupScriptMsg)
	base64SetupScript := base64.StdEncoding.EncodeToString([]byte(setupScript))
	params.SetupScript = &base64SetupScript

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")
		AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "test-repo-no-dotbrev"), ProvidedSetupScriptMsg)

		AssertRepoHasNumFiles(t, w, "/home/brev/workspace/test-repo-no-dotbrev/.brev/logs/archive", 3)

		resetMessage := "reset run"
		err = UpdateFile(w, fmt.Sprintf("%s/.brev/setup.sh", "test-repo-no-dotbrev"), fmt.Sprintf(" echo %s\n", resetMessage))
		assert.Nil(t, err)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")
		AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "test-repo-no-dotbrev"), resetMessage)
		AssertRepoHasNumFiles(t, w, "/home/brev/workspace/test-repo-no-dotbrev/.brev/logs/archive", 4)
	})
	assert.Nil(t, err)
}

func Test_ProvidedSetupNotRunProjBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)
	setupScript := fmt.Sprintf("echo %s ", ProvidedSetupScriptMsg)
	base64SetupScript := base64.StdEncoding.EncodeToString([]byte(setupScript))
	params.SetupScript = &base64SetupScript

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
		AssertFileNotContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "test-repo-dotbrev"), ProvidedSetupScriptMsg)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
		AssertFileNotContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "test-repo-dotbrev"), ProvidedSetupScriptMsg)
	})
	assert.Nil(t, err)
}

func Test_UnauthenticatedSSHKey(t *testing.T) {
	noauthKeys, err := GetUnauthedTestKeys()
	if !assert.Nil(t, err) {
		return
	}

	workskeys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(noauthKeys)

	params.WorkspaceBaseRepo = ""
	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Error(t, err)
		params.WorkspaceKeyPair = workskeys
		w.UpdateParams(params)
		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))
		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
	})
	assert.Nil(t, err)
}
