package whatsmiau

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
)

// ---------- Shared response shapes ----------

type GroupParticipantResponse struct {
	Jid          string `json:"jid"`
	Lid          string `json:"lid"`
	IsAdmin      bool   `json:"isAdmin"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
	DisplayName  string `json:"displayName,omitempty"`
	Error        int    `json:"error,omitempty"`
}

type GroupInfoResponse struct {
	Id                  string                     `json:"id"`
	Subject             string                     `json:"subject"`
	SubjectOwner        string                     `json:"subjectOwner,omitempty"`
	SubjectTime         int64                      `json:"subjectTime,omitempty"`
	Size                int                        `json:"size"`
	Creation            int64                      `json:"creation,omitempty"`
	Owner               string                     `json:"owner,omitempty"`
	Desc                string                     `json:"desc,omitempty"`
	DescId              string                     `json:"descId,omitempty"`
	Restrict            bool                       `json:"restrict"`
	Announce            bool                       `json:"announce"`
	IsCommunity         bool                       `json:"isCommunity"`
	IsCommunityAnnounce bool                       `json:"isCommunityAnnounce"`
	LinkedParent        string                     `json:"linkedParent,omitempty"`
	MemberAddMode       string                     `json:"memberAddMode,omitempty"`
	JoinApprovalMode    bool                       `json:"joinApprovalMode"`
	Ephemeral           uint32                     `json:"ephemeral,omitempty"`
	Participants        []GroupParticipantResponse `json:"participants,omitempty"`
}

func (s *Whatsmiau) buildGroupInfoResponse(ctx context.Context, instanceID string, g *types.GroupInfo, withParticipants bool) GroupInfoResponse {
	ownerJid, _ := s.GetJidLid(ctx, instanceID, g.OwnerJID)
	subjectOwnerJid, _ := s.GetJidLid(ctx, instanceID, g.NameSetBy)

	resp := GroupInfoResponse{
		Id:                  g.JID.String(),
		Subject:             g.Name,
		SubjectOwner:        subjectOwnerJid,
		SubjectTime:         g.NameSetAt.Unix(),
		Size:                g.ParticipantCount,
		Creation:            g.GroupCreated.Unix(),
		Owner:               ownerJid,
		Desc:                g.Topic,
		DescId:              g.TopicID,
		Restrict:            g.IsLocked,
		Announce:            g.IsAnnounce,
		IsCommunity:         g.IsParent,
		IsCommunityAnnounce: g.IsDefaultSubGroup && g.IsAnnounce,
		MemberAddMode:       string(g.MemberAddMode),
		JoinApprovalMode:    g.IsJoinApprovalRequired,
		Ephemeral:           g.DisappearingTimer,
	}
	if !g.LinkedParentJID.IsEmpty() {
		resp.LinkedParent = g.LinkedParentJID.String()
	}
	if g.ParticipantCount == 0 {
		resp.Size = len(g.Participants)
	}
	if withParticipants {
		resp.Participants = s.mapParticipants(ctx, instanceID, g.Participants)
	}
	return resp
}

func (s *Whatsmiau) mapParticipants(ctx context.Context, instanceID string, participants []types.GroupParticipant) []GroupParticipantResponse {
	out := make([]GroupParticipantResponse, 0, len(participants))
	for _, p := range participants {
		jid, lid := s.GetJidLid(ctx, instanceID, p.JID)
		out = append(out, GroupParticipantResponse{
			Jid:          jid,
			Lid:          lid,
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
			DisplayName:  p.DisplayName,
			Error:        p.Error,
		})
	}
	return out
}

// ---------- CreateGroup ----------

type CreateGroupRequest struct {
	InstanceID          string
	Subject             string
	Description         string
	Participants        []string
	PromoteParticipants bool
}

func (s *Whatsmiau) CreateGroup(ctx context.Context, req *CreateGroupRequest) (*GroupInfoResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	participantJids, err := s.numbersToJIDs(ctx, client, req.Participants, true)
	if err != nil {
		return nil, err
	}

	created, err := client.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:         req.Subject,
		Participants: participantJids,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	if req.Description != "" {
		if err := client.SetGroupDescription(ctx, created.JID, req.Description); err != nil {
			zap.L().Warn("group created but failed to set description", zap.Error(err), zap.String("group", created.JID.String()))
		}
	}

	if req.PromoteParticipants && len(participantJids) > 0 {
		if _, err := client.UpdateGroupParticipants(ctx, created.JID, participantJids, whatsmeow.ParticipantChangePromote); err != nil {
			zap.L().Warn("group created but failed to promote participants", zap.Error(err), zap.String("group", created.JID.String()))
		}
	}

	info, err := client.GetGroupInfo(ctx, created.JID)
	if err != nil {
		resp := s.buildGroupInfoResponse(ctx, req.InstanceID, created, true)
		return &resp, nil
	}
	resp := s.buildGroupInfoResponse(ctx, req.InstanceID, info, true)
	return &resp, nil
}

// ---------- SetGroupSubject ----------

type SetGroupSubjectRequest struct {
	InstanceID string
	GroupJID   *types.JID
	Subject    string
}

func (s *Whatsmiau) SetGroupSubject(ctx context.Context, req *SetGroupSubjectRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}
	return client.SetGroupName(ctx, *req.GroupJID, req.Subject)
}

// ---------- SetGroupPicture ----------

type SetGroupPictureRequest struct {
	InstanceID string
	GroupJID   *types.JID
	// Image is a base64-encoded JPEG (optionally with a data: prefix) or an http(s) URL.
	Image string
}

type SetGroupPictureResponse struct {
	PictureID string `json:"pictureId"`
}

func (s *Whatsmiau) SetGroupPicture(ctx context.Context, req *SetGroupPictureRequest) (*SetGroupPictureResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	if req.Image == "" {
		return nil, fmt.Errorf("image is required")
	}

	imgBytes, err := s.decodeImageInput(ctx, req.Image)
	if err != nil {
		return nil, err
	}

	pid, err := client.SetGroupPhoto(ctx, *req.GroupJID, imgBytes)
	if err != nil {
		return nil, err
	}
	return &SetGroupPictureResponse{PictureID: pid}, nil
}

func (s *Whatsmiau) decodeImageInput(ctx context.Context, input string) ([]byte, error) {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return s.fetchBytes(ctx, input)
	}
	if idx := strings.Index(input, ","); strings.HasPrefix(input, "data:") && idx > 0 {
		input = input[idx+1:]
	}
	decoded, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return nil, fmt.Errorf("image must be base64 or http(s) url: %w", err)
	}
	return decoded, nil
}

// ---------- SetGroupDescription ----------

type SetGroupDescriptionRequest struct {
	InstanceID  string
	GroupJID    *types.JID
	Description string
}

func (s *Whatsmiau) SetGroupDescription(ctx context.Context, req *SetGroupDescriptionRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}
	return client.SetGroupDescription(ctx, *req.GroupJID, req.Description)
}

// ---------- FindGroup ----------

type FindGroupRequest struct {
	InstanceID string
	GroupJID   *types.JID
}

func (s *Whatsmiau) FindGroup(ctx context.Context, req *FindGroupRequest) (*GroupInfoResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	info, err := client.GetGroupInfo(ctx, *req.GroupJID)
	if err != nil {
		return nil, err
	}
	resp := s.buildGroupInfoResponse(ctx, req.InstanceID, info, true)
	return &resp, nil
}

// ---------- FetchAllGroups ----------

type FetchAllGroupsRequest struct {
	InstanceID      string
	GetParticipants bool
}

func (s *Whatsmiau) FetchAllGroups(ctx context.Context, req *FetchAllGroupsRequest) ([]GroupInfoResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]GroupInfoResponse, 0, len(groups))
	for _, g := range groups {
		out = append(out, s.buildGroupInfoResponse(ctx, req.InstanceID, g, req.GetParticipants))
	}
	return out, nil
}

// ---------- FindParticipants ----------

type FindParticipantsRequest struct {
	InstanceID string
	GroupJID   *types.JID
}

type FindParticipantsResponse struct {
	Participants []GroupParticipantResponse `json:"participants"`
}

func (s *Whatsmiau) FindParticipants(ctx context.Context, req *FindParticipantsRequest) (*FindParticipantsResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	info, err := client.GetGroupInfo(ctx, *req.GroupJID)
	if err != nil {
		return nil, err
	}
	return &FindParticipantsResponse{
		Participants: s.mapParticipants(ctx, req.InstanceID, info.Participants),
	}, nil
}

// ---------- Invite Code ----------

type InviteCodeRequest struct {
	InstanceID string
	GroupJID   *types.JID
}

type InviteCodeResponse struct {
	InviteURL  string `json:"inviteUrl"`
	InviteCode string `json:"inviteCode"`
}

func (s *Whatsmiau) GetGroupInviteCode(ctx context.Context, req *InviteCodeRequest) (*InviteCodeResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	link, err := client.GetGroupInviteLink(ctx, *req.GroupJID, false)
	if err != nil {
		return nil, err
	}
	return &InviteCodeResponse{
		InviteURL:  link,
		InviteCode: strings.TrimPrefix(link, whatsmeow.InviteLinkPrefix),
	}, nil
}

func (s *Whatsmiau) RevokeGroupInviteCode(ctx context.Context, req *InviteCodeRequest) (*InviteCodeResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	link, err := client.GetGroupInviteLink(ctx, *req.GroupJID, true)
	if err != nil {
		return nil, err
	}
	return &InviteCodeResponse{
		InviteURL:  link,
		InviteCode: strings.TrimPrefix(link, whatsmeow.InviteLinkPrefix),
	}, nil
}

type InviteInfoRequest struct {
	InstanceID string
	Code       string
}

func (s *Whatsmiau) GetGroupInviteInfo(ctx context.Context, req *InviteInfoRequest) (*GroupInfoResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	info, err := client.GetGroupInfoFromLink(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	resp := s.buildGroupInfoResponse(ctx, req.InstanceID, info, true)
	return &resp, nil
}

type AcceptGroupInviteRequest struct {
	InstanceID string
	Code       string
}

type AcceptGroupInviteResponse struct {
	GroupJid string `json:"groupJid"`
}

func (s *Whatsmiau) AcceptGroupInvite(ctx context.Context, req *AcceptGroupInviteRequest) (*AcceptGroupInviteResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	jid, err := client.JoinGroupWithLink(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	return &AcceptGroupInviteResponse{GroupJid: jid.String()}, nil
}

// ---------- Send Invite to numbers ----------

type SendGroupInviteRequest struct {
	InstanceID  string
	GroupJID    *types.JID
	Description string
	Numbers     []string
}

type SendInviteResult struct {
	Number  string `json:"number"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type SendGroupInviteResponse struct {
	InviteURL string             `json:"inviteUrl"`
	Sent      []SendInviteResult `json:"sent"`
}

func (s *Whatsmiau) SendGroupInvite(ctx context.Context, req *SendGroupInviteRequest) (*SendGroupInviteResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	link, err := client.GetGroupInviteLink(ctx, *req.GroupJID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get invite link: %w", err)
	}

	body := link
	if req.Description != "" {
		body = req.Description + "\n\n" + link
	}

	results := make([]SendInviteResult, len(req.Numbers))
	var wg sync.WaitGroup
	for i, number := range req.Numbers {
		wg.Add(1)
		go func(i int, number string) {
			defer wg.Done()

			target, parseErr := normalizeUserJID(number)
			if parseErr != nil {
				results[i] = SendInviteResult{Number: number, Success: false, Error: parseErr.Error()}
				return
			}

			resolved := s.resolveJID(ctx, client, *target)
			_, sendErr := client.SendMessage(ctx, resolved, &waE2E.Message{
				Conversation: &body,
			})
			if sendErr != nil {
				results[i] = SendInviteResult{Number: number, Success: false, Error: sendErr.Error()}
				return
			}
			results[i] = SendInviteResult{Number: number, Success: true}
		}(i, number)
	}
	wg.Wait()

	return &SendGroupInviteResponse{
		InviteURL: link,
		Sent:      results,
	}, nil
}

// ---------- UpdateParticipant ----------

type UpdateParticipantRequest struct {
	InstanceID   string
	GroupJID     *types.JID
	Action       string
	Participants []string
}

type UpdateParticipantResponse struct {
	Participants []GroupParticipantResponse `json:"participants"`
}

func (s *Whatsmiau) UpdateGroupParticipant(ctx context.Context, req *UpdateParticipantRequest) (*UpdateParticipantResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	var action whatsmeow.ParticipantChange
	switch req.Action {
	case "add":
		action = whatsmeow.ParticipantChangeAdd
	case "remove":
		action = whatsmeow.ParticipantChangeRemove
	case "promote":
		action = whatsmeow.ParticipantChangePromote
	case "demote":
		action = whatsmeow.ParticipantChangeDemote
	default:
		return nil, fmt.Errorf("invalid action: %s", req.Action)
	}

	jids, err := s.numbersToJIDs(ctx, client, req.Participants, true)
	if err != nil {
		return nil, err
	}

	participants, err := client.UpdateGroupParticipants(ctx, *req.GroupJID, jids, action)
	if err != nil {
		return nil, err
	}
	return &UpdateParticipantResponse{
		Participants: s.mapParticipants(ctx, req.InstanceID, participants),
	}, nil
}

// ---------- UpdateSetting ----------

type UpdateGroupSettingRequest struct {
	InstanceID string
	GroupJID   *types.JID
	Action     string
}

func (s *Whatsmiau) UpdateGroupSetting(ctx context.Context, req *UpdateGroupSettingRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	switch req.Action {
	case "announcement":
		return client.SetGroupAnnounce(ctx, *req.GroupJID, true)
	case "not_announcement":
		return client.SetGroupAnnounce(ctx, *req.GroupJID, false)
	case "locked":
		return client.SetGroupLocked(ctx, *req.GroupJID, true)
	case "unlocked":
		return client.SetGroupLocked(ctx, *req.GroupJID, false)
	default:
		return fmt.Errorf("invalid action: %s", req.Action)
	}
}

// ---------- Toggle Ephemeral ----------

type ToggleEphemeralRequest struct {
	InstanceID string
	GroupJID   *types.JID
	Expiration uint32
}

func (s *Whatsmiau) ToggleGroupEphemeral(ctx context.Context, req *ToggleEphemeralRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}
	return client.SetDisappearingTimer(ctx, *req.GroupJID, time.Duration(req.Expiration)*time.Second, time.Time{})
}

// ---------- LeaveGroup ----------

type LeaveGroupRequest struct {
	InstanceID string
	GroupJID   *types.JID
}

func (s *Whatsmiau) LeaveGroup(ctx context.Context, req *LeaveGroupRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}
	return client.LeaveGroup(ctx, *req.GroupJID)
}

// ---------- Community: Create ----------

type CreateCommunityRequest struct {
	InstanceID  string
	Subject     string
	Description string
}

func (s *Whatsmiau) CreateCommunity(ctx context.Context, req *CreateCommunityRequest) (*GroupInfoResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	created, err := client.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name: req.Subject,
		GroupParent: types.GroupParent{
			IsParent: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create community: %w", err)
	}

	if req.Description != "" {
		if err := client.SetGroupDescription(ctx, created.JID, req.Description); err != nil {
			zap.L().Warn("community created but failed to set description", zap.Error(err), zap.String("community", created.JID.String()))
		}
	}

	info, err := client.GetGroupInfo(ctx, created.JID)
	if err != nil {
		resp := s.buildGroupInfoResponse(ctx, req.InstanceID, created, false)
		return &resp, nil
	}
	resp := s.buildGroupInfoResponse(ctx, req.InstanceID, info, false)
	return &resp, nil
}

// ---------- Community: Create SubGroup ----------

type CreateSubGroupRequest struct {
	InstanceID   string
	Subject      string
	ParentJID    *types.JID
	Participants []string
}

func (s *Whatsmiau) CreateCommunitySubGroup(ctx context.Context, req *CreateSubGroupRequest) (*GroupInfoResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	participantJids, err := s.numbersToJIDs(ctx, client, req.Participants, true)
	if err != nil {
		return nil, err
	}

	created, err := client.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:         req.Subject,
		Participants: participantJids,
		GroupLinkedParent: types.GroupLinkedParent{
			LinkedParentJID: *req.ParentJID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sub group: %w", err)
	}

	info, err := client.GetGroupInfo(ctx, created.JID)
	if err != nil {
		resp := s.buildGroupInfoResponse(ctx, req.InstanceID, created, true)
		return &resp, nil
	}
	resp := s.buildGroupInfoResponse(ctx, req.InstanceID, info, true)
	return &resp, nil
}

// ---------- Community: Link / Unlink ----------

type LinkGroupRequest struct {
	InstanceID string
	ParentJID  *types.JID
	ChildJID   *types.JID
}

func (s *Whatsmiau) LinkCommunityGroup(ctx context.Context, req *LinkGroupRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	err := client.LinkGroup(ctx, *req.ParentJID, *req.ChildJID)
	if err != nil {
		if errors.Is(err, whatsmeow.ErrIQNotAuthorized) || errors.Is(err, whatsmeow.ErrIQForbidden) {
			return fmt.Errorf("you must be an admin of the parent community to link groups: %w", err)
		}
		return err
	}
	return nil
}

func (s *Whatsmiau) UnlinkCommunityGroup(ctx context.Context, req *LinkGroupRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	err := client.UnlinkGroup(ctx, *req.ParentJID, *req.ChildJID)
	if err != nil {
		if errors.Is(err, whatsmeow.ErrIQNotAuthorized) || errors.Is(err, whatsmeow.ErrIQForbidden) {
			return fmt.Errorf("you must be an admin of the parent community to unlink groups: %w", err)
		}
		return err
	}
	return nil
}

// ---------- Community: SubGroups list ----------

type SubGroupsRequest struct {
	InstanceID   string
	CommunityJID *types.JID
}

type SubGroupResponse struct {
	Id                string `json:"id"`
	Subject           string `json:"subject"`
	SubjectTime       int64  `json:"subjectTime,omitempty"`
	IsDefaultSubGroup bool   `json:"isDefaultSubGroup"`
}

func (s *Whatsmiau) GetCommunitySubGroups(ctx context.Context, req *SubGroupsRequest) ([]SubGroupResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	subs, err := client.GetSubGroups(ctx, *req.CommunityJID)
	if err != nil {
		return nil, err
	}

	out := make([]SubGroupResponse, 0, len(subs))
	for _, sg := range subs {
		out = append(out, SubGroupResponse{
			Id:                sg.JID.String(),
			Subject:           sg.Name,
			SubjectTime:       sg.NameSetAt.Unix(),
			IsDefaultSubGroup: sg.IsDefaultSubGroup,
		})
	}
	return out, nil
}

// ---------- Community: Linked groups participants ----------

type LinkedGroupsParticipantsRequest struct {
	InstanceID   string
	CommunityJID *types.JID
}

type LinkedParticipantResponse struct {
	Jid string `json:"jid"`
	Lid string `json:"lid"`
}

func (s *Whatsmiau) GetCommunityLinkedGroupsParticipants(ctx context.Context, req *LinkedGroupsParticipantsRequest) ([]LinkedParticipantResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	participants, err := client.GetLinkedGroupsParticipants(ctx, *req.CommunityJID)
	if err != nil {
		return nil, err
	}

	out := make([]LinkedParticipantResponse, 0, len(participants))
	for _, p := range participants {
		jid, lid := s.GetJidLid(ctx, req.InstanceID, p)
		out = append(out, LinkedParticipantResponse{Jid: jid, Lid: lid})
	}
	return out, nil
}

// ---------- Community: Settings ----------

type SetJoinApprovalModeRequest struct {
	InstanceID   string
	CommunityJID *types.JID
	Mode         bool
}

func (s *Whatsmiau) SetCommunityJoinApprovalMode(ctx context.Context, req *SetJoinApprovalModeRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}
	return client.SetGroupJoinApprovalMode(ctx, *req.CommunityJID, req.Mode)
}

type SetMemberAddModeRequest struct {
	InstanceID   string
	CommunityJID *types.JID
	Mode         string
}

func (s *Whatsmiau) SetCommunityMemberAddMode(ctx context.Context, req *SetMemberAddModeRequest) error {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	var mode types.GroupMemberAddMode
	switch req.Mode {
	case "admin_add":
		mode = types.GroupMemberAddModeAdmin
	case "all_member_add":
		mode = types.GroupMemberAddModeAllMember
	default:
		return fmt.Errorf("invalid mode: %s", req.Mode)
	}
	return client.SetGroupMemberAddMode(ctx, *req.CommunityJID, mode)
}

// ---------- Community: Request participants ----------

type RequestParticipantsListRequest struct {
	InstanceID   string
	CommunityJID *types.JID
}

type RequestParticipantResponse struct {
	Jid         string `json:"jid"`
	Lid         string `json:"lid"`
	RequestedAt int64  `json:"requestedAt"`
}

func (s *Whatsmiau) GetCommunityRequestParticipants(ctx context.Context, req *RequestParticipantsListRequest) ([]RequestParticipantResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	requests, err := client.GetGroupRequestParticipants(ctx, *req.CommunityJID)
	if err != nil {
		return nil, err
	}

	out := make([]RequestParticipantResponse, 0, len(requests))
	for _, r := range requests {
		jid, lid := s.GetJidLid(ctx, req.InstanceID, r.JID)
		out = append(out, RequestParticipantResponse{
			Jid:         jid,
			Lid:         lid,
			RequestedAt: r.RequestedAt.Unix(),
		})
	}
	return out, nil
}

type UpdateRequestParticipantsRequest struct {
	InstanceID   string
	CommunityJID *types.JID
	Action       string
	Participants []string
}

func (s *Whatsmiau) UpdateCommunityRequestParticipants(ctx context.Context, req *UpdateRequestParticipantsRequest) (*UpdateParticipantResponse, error) {
	client, ok := s.clients.Load(req.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	var action whatsmeow.ParticipantRequestChange
	switch req.Action {
	case "approve":
		action = whatsmeow.ParticipantChangeApprove
	case "reject":
		action = whatsmeow.ParticipantChangeReject
	default:
		return nil, fmt.Errorf("invalid action: %s", req.Action)
	}

	jids, err := s.numbersToJIDs(ctx, client, req.Participants, true)
	if err != nil {
		return nil, err
	}

	participants, err := client.UpdateGroupRequestParticipants(ctx, *req.CommunityJID, jids, action)
	if err != nil {
		return nil, err
	}
	return &UpdateParticipantResponse{
		Participants: s.mapParticipants(ctx, req.InstanceID, participants),
	}, nil
}

// ---------- helpers ----------

// numbersToJIDs converts a list of phone numbers (raw or jid-format) into types.JID.
// When resolve is true, applies brazilian-alternate resolution via resolveJID.
func (s *Whatsmiau) numbersToJIDs(ctx context.Context, client *whatsmeow.Client, numbers []string, resolve bool) ([]types.JID, error) {
	out := make([]types.JID, 0, len(numbers))
	for _, n := range numbers {
		jid, err := normalizeUserJID(n)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q: %w", n, err)
		}
		if resolve {
			resolved := s.resolveJID(ctx, client, *jid)
			out = append(out, resolved)
		} else {
			out = append(out, *jid)
		}
	}
	return out, nil
}

// normalizeUserJID accepts "55..." or "55...@s.whatsapp.net" or "lid@lid" and returns a types.JID.
// Differs from controllers.numberToJid by not enforcing minimum length (already validated upstream).
func normalizeUserJID(input string) (*types.JID, error) {
	if input == "" {
		return nil, fmt.Errorf("empty number")
	}
	if !strings.Contains(input, "@") {
		input += "@" + types.DefaultUserServer
	}
	jid, err := types.ParseJID(input)
	if err != nil {
		return nil, err
	}
	return &jid, nil
}
