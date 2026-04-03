package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/openzerg/nydus/gen/nydus/v1"
	mutalisk "github.com/openzerg/nydus/internal/mutalisk"
	"github.com/openzerg/nydus/internal/store"
)

// InstanceIPResolver is a function that returns the IP of a given instance ID.
// Returns empty string if unknown.
type InstanceIPResolver func(instanceID string) string

type ChatroomHandler struct {
	store               *store.Store
	messageSubscription *store.MessageSubscriptionManager
	getInstanceIP       InstanceIPResolver
	mutaliskPort        int
}

func NewChatroomHandler(s *store.Store, msm *store.MessageSubscriptionManager, getIP InstanceIPResolver, mutaliskPort int) *ChatroomHandler {
	return &ChatroomHandler{
		store:               s,
		messageSubscription: msm,
		getInstanceIP:       getIP,
		mutaliskPort:        mutaliskPort,
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *ChatroomHandler) CreateChatroom(ctx context.Context, req *connect.Request[nydusv1.CreateChatroomRequest]) (*connect.Response[nydusv1.Chatroom], error) {
	now := time.Now().Unix()
	chatroomID := generateID()

	chatroom := &store.Chatroom{
		ID:        chatroomID,
		Name:      req.Msg.Name,
		CreatedBy: req.Msg.CreatedBy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.store.CreateChatroom(chatroom); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create chatroom: %w", err))
	}

	for _, m := range req.Msg.Members {
		member := &store.Member{
			ID:         generateID(),
			ChatroomID: chatroomID,
			MemberID:   m.MemberId,
			MemberType: m.MemberType,
			Role:       m.Role,
			JoinedAt:   now,
		}
		if err := h.store.AddMember(member); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add member: %w", err))
		}
	}

	return connect.NewResponse(&nydusv1.Chatroom{
		Id:        chatroom.ID,
		Name:      chatroom.Name,
		CreatedBy: chatroom.CreatedBy,
		CreatedAt: chatroom.CreatedAt,
		UpdatedAt: chatroom.UpdatedAt,
	}), nil
}

func (h *ChatroomHandler) GetChatroom(ctx context.Context, req *connect.Request[nydusv1.GetChatroomRequest]) (*connect.Response[nydusv1.Chatroom], error) {
	chatroom, err := h.store.GetChatroom(req.Msg.ChatroomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if chatroom == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("chatroom not found"))
	}

	return connect.NewResponse(&nydusv1.Chatroom{
		Id:        chatroom.ID,
		Name:      chatroom.Name,
		CreatedBy: chatroom.CreatedBy,
		CreatedAt: chatroom.CreatedAt,
		UpdatedAt: chatroom.UpdatedAt,
	}), nil
}

func (h *ChatroomHandler) UpdateChatroom(ctx context.Context, req *connect.Request[nydusv1.UpdateChatroomRequest]) (*connect.Response[nydusv1.Chatroom], error) {
	if req.Msg.Name != nil {
		if err := h.store.UpdateChatroom(req.Msg.ChatroomId, *req.Msg.Name); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	chatroom, err := h.store.GetChatroom(req.Msg.ChatroomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if chatroom == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("chatroom not found"))
	}

	return connect.NewResponse(&nydusv1.Chatroom{
		Id:        chatroom.ID,
		Name:      chatroom.Name,
		CreatedBy: chatroom.CreatedBy,
		CreatedAt: chatroom.CreatedAt,
		UpdatedAt: chatroom.UpdatedAt,
	}), nil
}

func (h *ChatroomHandler) DeleteChatroom(ctx context.Context, req *connect.Request[nydusv1.DeleteChatroomRequest]) (*connect.Response[nydusv1.Empty], error) {
	if err := h.store.DeleteChatroom(req.Msg.ChatroomId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&nydusv1.Empty{}), nil
}

func (h *ChatroomHandler) ListChatrooms(ctx context.Context, req *connect.Request[nydusv1.ListChatroomsRequest]) (*connect.Response[nydusv1.ListChatroomsResponse], error) {
	limit := req.Msg.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	chatrooms, total, err := h.store.ListChatrooms(req.Msg.MemberId, req.Msg.MemberType, int(limit), int(req.Msg.Offset))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var result []*nydusv1.Chatroom
	for _, c := range chatrooms {
		result = append(result, &nydusv1.Chatroom{
			Id:        c.ID,
			Name:      c.Name,
			CreatedBy: c.CreatedBy,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}

	return connect.NewResponse(&nydusv1.ListChatroomsResponse{
		Chatrooms: result,
		Total:     int32(total),
	}), nil
}

func (h *ChatroomHandler) AddMember(ctx context.Context, req *connect.Request[nydusv1.AddMemberRequest]) (*connect.Response[nydusv1.Member], error) {
	chatroom, err := h.store.GetChatroom(req.Msg.ChatroomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if chatroom == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("chatroom not found"))
	}

	member := &store.Member{
		ID:         generateID(),
		ChatroomID: req.Msg.ChatroomId,
		MemberID:   req.Msg.MemberId,
		MemberType: req.Msg.MemberType,
		Role:       req.Msg.Role,
		JoinedAt:   time.Now().Unix(),
	}

	if err := h.store.AddMember(member); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add member: %w", err))
	}

	return connect.NewResponse(&nydusv1.Member{
		Id:         member.ID,
		ChatroomId: member.ChatroomID,
		MemberId:   member.MemberID,
		MemberType: member.MemberType,
		Role:       member.Role,
		JoinedAt:   member.JoinedAt,
	}), nil
}

func (h *ChatroomHandler) RemoveMember(ctx context.Context, req *connect.Request[nydusv1.RemoveMemberRequest]) (*connect.Response[nydusv1.Empty], error) {
	if err := h.store.RemoveMember(req.Msg.ChatroomId, req.Msg.MemberId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&nydusv1.Empty{}), nil
}

func (h *ChatroomHandler) ListMembers(ctx context.Context, req *connect.Request[nydusv1.ListMembersRequest]) (*connect.Response[nydusv1.ListMembersResponse], error) {
	members, err := h.store.ListMembers(req.Msg.ChatroomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var result []*nydusv1.Member
	for _, m := range members {
		result = append(result, &nydusv1.Member{
			Id:         m.ID,
			ChatroomId: m.ChatroomID,
			MemberId:   m.MemberID,
			MemberType: m.MemberType,
			Role:       m.Role,
			JoinedAt:   m.JoinedAt,
		})
	}

	return connect.NewResponse(&nydusv1.ListMembersResponse{
		Members: result,
	}), nil
}

func (h *ChatroomHandler) UpdateMemberRole(ctx context.Context, req *connect.Request[nydusv1.UpdateMemberRoleRequest]) (*connect.Response[nydusv1.Member], error) {
	if err := h.store.UpdateMemberRole(req.Msg.ChatroomId, req.Msg.MemberId, req.Msg.Role); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	member, err := h.store.GetMember(req.Msg.ChatroomId, req.Msg.MemberId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if member == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("member not found"))
	}

	return connect.NewResponse(&nydusv1.Member{
		Id:         member.ID,
		ChatroomId: member.ChatroomID,
		MemberId:   member.MemberID,
		MemberType: member.MemberType,
		Role:       member.Role,
		JoinedAt:   member.JoinedAt,
	}), nil
}

func (h *ChatroomHandler) SendMessage(ctx context.Context, req *connect.Request[nydusv1.SendMessageRequest]) (*connect.Response[nydusv1.Message], error) {
	chatroom, err := h.store.GetChatroom(req.Msg.ChatroomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if chatroom == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("chatroom not found"))
	}

	member, err := h.store.GetMember(req.Msg.ChatroomId, req.Msg.SenderId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if member == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("sender not a member of chatroom"))
	}

	message := &store.Message{
		ID:         generateID(),
		ChatroomID: req.Msg.ChatroomId,
		SenderID:   req.Msg.SenderId,
		SenderType: req.Msg.SenderType,
		Content:    req.Msg.Content,
		ReplyTo:    req.Msg.ReplyTo,
		CreatedAt:  time.Now().Unix(),
	}

	if err := h.store.CreateMessage(message); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create message: %w", err))
	}

	h.messageSubscription.Broadcast(req.Msg.ChatroomId, message)

	// If the message is from a human, dispatch it to all instance (agent) members
	// asynchronously so as not to block the response.
	if req.Msg.SenderType == "human" {
		go h.dispatchToAgents(req.Msg.ChatroomId, req.Msg.SenderId, req.Msg.Content)
	}

	return connect.NewResponse(&nydusv1.Message{
		Id:         message.ID,
		ChatroomId: message.ChatroomID,
		SenderId:   message.SenderID,
		SenderType: message.SenderType,
		Content:    message.Content,
		ReplyTo:    message.ReplyTo,
		CreatedAt:  message.CreatedAt,
	}), nil
}

// dispatchToAgents finds all instance members of the chatroom and sends the
// human message to their Mutalisk sessions via AddMessageToSession.
func (h *ChatroomHandler) dispatchToAgents(chatroomID, senderID, content string) {
	instanceMembers, err := h.store.GetInstanceMembers(chatroomID)
	if err != nil {
		log.Printf("[nydus] dispatchToAgents: failed to list instance members for chatroom %s: %v", chatroomID, err)
		return
	}

	for _, m := range instanceMembers {
		instanceID := m.MemberID
		go h.dispatchToAgent(chatroomID, instanceID, senderID, content)
	}
}

// dispatchToAgent sends a message to a single agent instance's Mutalisk session.
func (h *ChatroomHandler) dispatchToAgent(chatroomID, instanceID, senderID, content string) {
	// Build Mutalisk base URL from instance IP
	ip := h.getInstanceIP(instanceID)
	if ip == "" {
		log.Printf("[nydus] dispatchToAgent: unknown IP for instance %s, skipping", instanceID)
		return
	}

	baseURL := fmt.Sprintf("http://%s:%d", ip, h.mutaliskPort)
	client := mutalisk.NewClient(&http.Client{}, baseURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// external_id = chatroom_id:instance_id (unique session per chatroom per agent)
	externalID := fmt.Sprintf("%s:%s", chatroomID, instanceID)
	sessionName := fmt.Sprintf("chatroom-%s", chatroomID)

	// Get or create a session for this chatroom+instance pair
	session, err := client.GetOrCreateSessionByExternalId(ctx, externalID, sessionName, "", "", "")
	if err != nil {
		log.Printf("[nydus] dispatchToAgent: failed to get/create session for instance %s in chatroom %s: %v", instanceID, chatroomID, err)
		return
	}

	sessionID := session.GetId()

	// Record session mapping in store (upsert)
	existing, _ := h.store.GetChatroomSession(chatroomID, instanceID)
	if existing == nil {
		_ = h.store.CreateChatroomSession(&store.ChatroomSession{
			ID:         generateID(),
			ChatroomID: chatroomID,
			InstanceID: instanceID,
			SessionID:  sessionID,
			ExternalID: externalID,
			CreatedAt:  time.Now().Unix(),
		})
	} else if existing.SessionID != sessionID {
		_ = h.store.UpdateChatroomSession(chatroomID, instanceID, sessionID)
	}

	// Add the human message to the agent's session
	_, err = client.AddMessageToSession(ctx, sessionID, "user", fmt.Sprintf("[%s]: %s", senderID, content))
	if err != nil {
		log.Printf("[nydus] dispatchToAgent: failed to add message to session %s for instance %s: %v", sessionID, instanceID, err)
		return
	}

	log.Printf("[nydus] dispatchToAgent: dispatched message to instance %s session %s", instanceID, sessionID)
}

func (h *ChatroomHandler) GetMessageHistory(ctx context.Context, req *connect.Request[nydusv1.GetMessageHistoryRequest]) (*connect.Response[nydusv1.GetMessageHistoryResponse], error) {
	limit := req.Msg.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	messages, hasMore, err := h.store.GetMessageHistory(req.Msg.ChatroomId, req.Msg.BeforeId, int(limit))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var result []*nydusv1.Message
	for _, m := range messages {
		result = append(result, &nydusv1.Message{
			Id:         m.ID,
			ChatroomId: m.ChatroomID,
			SenderId:   m.SenderID,
			SenderType: m.SenderType,
			Content:    m.Content,
			ReplyTo:    m.ReplyTo,
			CreatedAt:  m.CreatedAt,
		})
	}

	return connect.NewResponse(&nydusv1.GetMessageHistoryResponse{
		Messages: result,
		HasMore:  hasMore,
	}), nil
}

func (h *ChatroomHandler) SubscribeMessages(ctx context.Context, req *connect.Request[nydusv1.SubscribeMessagesRequest], stream *connect.ServerStream[nydusv1.Message]) error {
	chatroom, err := h.store.GetChatroom(req.Msg.ChatroomId)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if chatroom == nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("chatroom not found"))
	}

	member, err := h.store.GetMember(req.Msg.ChatroomId, req.Msg.InstanceId)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if member == nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not a member of chatroom"))
	}

	sub := h.messageSubscription.Add(req.Msg.ChatroomId, req.Msg.InstanceId)
	defer h.messageSubscription.Remove(req.Msg.ChatroomId, req.Msg.InstanceId)

	for {
		select {
		case message := <-sub.Channel:
			if err := stream.Send(&nydusv1.Message{
				Id:         message.ID,
				ChatroomId: message.ChatroomID,
				SenderId:   message.SenderID,
				SenderType: message.SenderType,
				Content:    message.Content,
				ReplyTo:    message.ReplyTo,
				CreatedAt:  message.CreatedAt,
			}); err != nil {
				return err
			}
		case <-sub.Done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (h *ChatroomHandler) GetChatroomSessions(ctx context.Context, req *connect.Request[nydusv1.GetChatroomSessionsRequest]) (*connect.Response[nydusv1.GetChatroomSessionsResponse], error) {
	sessions, err := h.store.GetChatroomSessions(req.Msg.ChatroomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var result []*nydusv1.ChatroomSession
	for _, s := range sessions {
		result = append(result, &nydusv1.ChatroomSession{
			ChatroomId: s.ChatroomID,
			InstanceId: s.InstanceID,
			SessionId:  s.SessionID,
			ExternalId: s.ExternalID,
		})
	}

	return connect.NewResponse(&nydusv1.GetChatroomSessionsResponse{
		Sessions: result,
	}), nil
}
