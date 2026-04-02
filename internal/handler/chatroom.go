package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/openzerg/nydus/gen/nydus/v1"
	"github.com/openzerg/nydus/internal/store"
)

type ChatroomHandler struct {
	store               *store.Store
	messageSubscription *store.MessageSubscriptionManager
}

func NewChatroomHandler(s *store.Store, msm *store.MessageSubscriptionManager) *ChatroomHandler {
	return &ChatroomHandler{
		store:               s,
		messageSubscription: msm,
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
