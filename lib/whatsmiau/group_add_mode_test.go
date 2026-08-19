package whatsmiau

import (
	"context"
	"errors"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

type fakeGroupAddModeClient struct {
	info          *types.GroupInfo
	infoErr       error
	setErr        error
	setJID        types.JID
	setMode       types.CommunityGroupAddMode
	setCallCount  int
	infoCallCount int
}

func (f *fakeGroupAddModeClient) GetGroupInfo(context.Context, types.JID) (*types.GroupInfo, error) {
	f.infoCallCount++
	return f.info, f.infoErr
}

func (f *fakeGroupAddModeClient) SetCommunityGroupAddMode(_ context.Context, jid types.JID, mode types.CommunityGroupAddMode) error {
	f.setCallCount++
	f.setJID = jid
	f.setMode = mode
	return f.setErr
}

func TestSetGroupAddModeRestrictsCommunityToAdmins(t *testing.T) {
	community := types.NewJID("community", types.GroupServer)
	client := &fakeGroupAddModeClient{
		info: &types.GroupInfo{JID: community, GroupParent: types.GroupParent{IsParent: true}},
	}

	err := setGroupAddMode(context.Background(), client, community, "admin_add")
	if err != nil {
		t.Fatalf("setGroupAddMode returned an unexpected error: %v", err)
	}
	if client.setJID != community {
		t.Fatalf("set target = %s, want community %s", client.setJID, community)
	}
	if client.setMode != types.CommunityGroupAddModeAdmin {
		t.Fatalf("set mode = %q, want %q", client.setMode, types.CommunityGroupAddModeAdmin)
	}
}

func TestSetGroupAddModeAllowsAllCommunityMembers(t *testing.T) {
	community := types.NewJID("community", types.GroupServer)
	client := &fakeGroupAddModeClient{
		info: &types.GroupInfo{JID: community, GroupParent: types.GroupParent{IsParent: true}},
	}

	err := setGroupAddMode(context.Background(), client, community, "all_member_add")
	if err != nil {
		t.Fatalf("setGroupAddMode returned an unexpected error: %v", err)
	}
	if client.setMode != types.CommunityGroupAddModeAllMember {
		t.Fatalf("set mode = %q, want %q", client.setMode, types.CommunityGroupAddModeAllMember)
	}
}

func TestSetGroupAddModeRejectsNonCommunity(t *testing.T) {
	group := types.NewJID("group", types.GroupServer)
	client := &fakeGroupAddModeClient{info: &types.GroupInfo{JID: group}}

	err := setGroupAddMode(context.Background(), client, group, "admin_add")
	if !errors.Is(err, ErrGroupAddModeRequiresCommunity) {
		t.Fatalf("error = %v, want ErrGroupAddModeRequiresCommunity", err)
	}
	if client.setCallCount != 0 {
		t.Fatalf("SetCommunityGroupAddMode calls = %d, want 0", client.setCallCount)
	}
}

func TestSetGroupAddModeRejectsInvalidModeBeforeNetworkCalls(t *testing.T) {
	client := &fakeGroupAddModeClient{}
	err := setGroupAddMode(context.Background(), client, types.NewJID("community", types.GroupServer), "everyone")
	if err == nil {
		t.Fatal("setGroupAddMode returned nil for an invalid mode")
	}
	if client.infoCallCount != 0 || client.setCallCount != 0 {
		t.Fatalf("network calls were made for invalid mode: info=%d set=%d", client.infoCallCount, client.setCallCount)
	}
}

func TestSetGroupAddModePropagatesMetadataError(t *testing.T) {
	wantErr := errors.New("metadata unavailable")
	client := &fakeGroupAddModeClient{infoErr: wantErr}

	err := setGroupAddMode(context.Background(), client, types.NewJID("community", types.GroupServer), "admin_add")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
	if client.setCallCount != 0 {
		t.Fatalf("SetCommunityGroupAddMode calls = %d, want 0", client.setCallCount)
	}
}
