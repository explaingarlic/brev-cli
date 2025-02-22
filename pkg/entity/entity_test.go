package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLocalIdentifier(t *testing.T) {
	w := Workspace{DNS: "test-rand-5566-org.brev.sh", ID: "123456789", Name: "test-rand"}
	assert.Equal(t, WorkspaceLocalID("test-rand-6789"), w.GetLocalIdentifier())
}

func TestGetLocalIdentifierClean(t *testing.T) {
	// safest https://www.saveonhosting.com/scripts/index.php?rp=/knowledgebase/52/What-are-the-valid-characters-for-a-domain-name-and-how-long-can-a-domain-name-be.html
	correctID := WorkspaceLocalID("test-rand-6789")
	w := Workspace{DNS: "test-rand-6789-org.brev.sh", Name: "test-rand", ID: "123456789"}
	assert.Equal(t, correctID, w.GetLocalIdentifier())

	w.Name = ":./\\\"'test rand"
	assert.Equal(t, correctID, w.GetLocalIdentifier())
}

// NADER IS SO FUCKING SORRY FOR DOING THIS TWICE BUT I HAVE NO CLUE WHERE THIS HELPER FUNCTION SHOULD GO SO ITS COPY/PASTED ELSEWHERE
// IF YOU MODIFY IT MODIFY IT EVERYWHERE OR PLEASE PUT IT IN ITS PROPER PLACE. thank you you're the best <3
func WorkspacesFromWorkspaceWithMeta(wwm []WorkspaceWithMeta) []Workspace {
	var workspaces []Workspace

	for _, v := range wwm {
		workspaces = append(workspaces, Workspace{
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

func TestGetLocalIdentifierDeterminism(t *testing.T) {
	w1 := WorkspaceWithMeta{Workspace: Workspace{ID: "123456789", DNS: "main-6789-org.brev.sh", Name: "main", CreatedByUserID: "user"}}
	w2 := WorkspaceWithMeta{Workspace: Workspace{ID: "212345678", DNS: "main-5678-org.brev.sh", Name: "main", CreatedByUserID: "user"}}

	// same id must be returned across time
	w1CorrectID := WorkspaceLocalID("main-6789")
	w2CorrectID := WorkspaceLocalID("main-5678")

	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier())
	assert.Equal(t, w2CorrectID, w2.GetLocalIdentifier())

	// sometime later -- re-arranged
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier())
	assert.Equal(t, w2CorrectID, w2.GetLocalIdentifier())

	// sometime later -- w2 deleted
	assert.Equal(t, w1CorrectID, w1.GetLocalIdentifier())

	// sometime later -- user changes name // hmm undefined behavior
	w1.Name = "new name"
	assert.NotEqual(t, w1CorrectID, w1.GetLocalIdentifier())
}

func TestNewVirtualProjectOne(t *testing.T) {
	ps := NewVirtualProjects([]Workspace{{
		ID:              "1",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}})
	if !assert.Len(t, ps, 1) {
		return
	}
	assert.Equal(t, ps[0].Name, "hi")
	assert.Equal(t, ps[0].GitURL, "git://hi")
	assert.Equal(t, 1, ps[0].GetUniqueUserCount())
	assert.Len(t, ps[0].GetUserWorkspaces("me"), 1)
}

func TestNewVirtualProjectNone(t *testing.T) {
	ps := NewVirtualProjects([]Workspace{})
	assert.Len(t, ps, 0)
}

func TestNewVirtualProjectTwoDiffRepo(t *testing.T) {
	ps := NewVirtualProjects([]Workspace{{
		ID:              "1",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}, {
		ID:              "2",
		Name:            "bye",
		GitRepo:         "git://bye",
		CreatedByUserID: "other",
	}})
	if !assert.Len(t, ps, 2) {
		return
	}
	assert.Equal(t, ps[0].Name, "hi")
	assert.Equal(t, ps[0].GitURL, "git://hi")
	assert.Equal(t, 1, ps[0].GetUniqueUserCount())
	assert.Len(t, ps[0].GetUserWorkspaces("me"), 1)

	assert.Equal(t, ps[1].Name, "bye")
	assert.Equal(t, ps[1].GitURL, "git://bye")
	assert.Equal(t, 1, ps[1].GetUniqueUserCount())
	assert.Len(t, ps[1].GetUserWorkspaces("other"), 1)
}

func TestNewVirtualProjectTwoSameRepo(t *testing.T) {
	ps := NewVirtualProjects([]Workspace{{
		ID:              "1",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}, {
		ID:              "2",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "other",
	}})
	if !assert.Len(t, ps, 1) {
		return
	}
	assert.Equal(t, ps[0].Name, "hi")
	assert.Equal(t, ps[0].GitURL, "git://hi")
	assert.Equal(t, 2, ps[0].GetUniqueUserCount())
	assert.Len(t, ps[0].GetUserWorkspaces("me"), 1)
	assert.Len(t, ps[0].GetUserWorkspaces("other"), 1)
}

func TestNewVirtualProjectTwoSameRepoSameUser(t *testing.T) {
	ps := NewVirtualProjects([]Workspace{{
		ID:              "1",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}, {
		ID:              "2",
		Name:            "hi",
		GitRepo:         "git://hi",
		CreatedByUserID: "me",
	}})
	if !assert.Len(t, ps, 1) {
		return
	}
	assert.Equal(t, ps[0].Name, "hi")
	assert.Equal(t, ps[0].GitURL, "git://hi")
	assert.Equal(t, 1, ps[0].GetUniqueUserCount())
	assert.Len(t, ps[0].GetUserWorkspaces("me"), 2)
	assert.Len(t, ps[0].GetUserWorkspaces("other"), 0)
}
