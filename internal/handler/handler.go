package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"connectrpc.com/connect"
	nydusv1 "github.com/openzerg/common/gen/nydus/v1"
	nydusv1connect "github.com/openzerg/common/gen/nydus/v1/nydusv1connect"

	"github.com/openzerg/nydus/internal/store"
)

var _ nydusv1connect.NydusServiceHandler = (*Handler)(nil)

type Handler struct {
	store    *store.Store
	events   *eventFanout
	messages *messageFanout
}

func New(st *store.Store) *Handler {
	return &Handler{
		store:    st,
		events:   newEventFanout(),
		messages: newMessageFanout(),
	}
}

// ── Event Bus ────────────────────────────────────────────────────────────────

func (h *Handler) PublishEvent(ctx context.Context, req *connect.Request[nydusv1.PublishEventRequest]) (*connect.Response[nydusv1.PublishEventResponse], error) {
	id, err := randomHex(8)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	now := time.Now().Unix()
	event := &nydusv1.Event{
		EventId:          id,
		EventType:        req.Msg.EventType,
		SourceInstanceId: req.Msg.SourceInstanceId,
		Timestamp:        now,
		Data:             req.Msg.Data,
	}
	if err := h.store.SaveEvent(event); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	h.events.publish(event)
	return connect.NewResponse(&nydusv1.PublishEventResponse{EventId: id, Timestamp: now}), nil
}

func (h *Handler) Subscribe(ctx context.Context, req *connect.Request[nydusv1.SubscribeRequest], stream *connect.ServerStream[nydusv1.Event]) error {
	instanceID := req.Msg.InstanceId
	if instanceID == "" {
		var err error
		if instanceID, err = randomHex(8); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := h.store.UpsertSubscriber(instanceID, "", req.Msg.EventTypes); err != nil {
		log.Printf("[nydus] save subscriber: %v", err)
	}

	ch := h.events.subscribe(instanceID, req.Msg.EventTypes)
	defer func() {
		h.events.unsubscribe(instanceID)
		_ = h.store.DeleteSubscriber(instanceID)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

func (h *Handler) Unsubscribe(ctx context.Context, req *connect.Request[nydusv1.UnsubscribeRequest]) (*connect.Response[nydusv1.Empty], error) {
	h.events.unsubscribe(req.Msg.InstanceId)
	_ = h.store.DeleteSubscriber(req.Msg.InstanceId)
	return connect.NewResponse(&nydusv1.Empty{}), nil
}

func (h *Handler) ListSubscribers(ctx context.Context, req *connect.Request[nydusv1.ListSubscribersRequest]) (*connect.Response[nydusv1.ListSubscribersResponse], error) {
	subs, err := h.store.ListSubscribers()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&nydusv1.ListSubscribersResponse{Subscribers: subs}), nil
}

func (h *Handler) GetEventHistory(ctx context.Context, req *connect.Request[nydusv1.GetEventHistoryRequest]) (*connect.Response[nydusv1.GetEventHistoryResponse], error) {
	events, total, err := h.store.GetEventHistory(req.Msg.EventType, req.Msg.Limit, req.Msg.Offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&nydusv1.GetEventHistoryResponse{Events: events, Total: total}), nil
}

// ── Chatroom ─────────────────────────────────────────────────────────────────

func (h *Handler) CreateChatroom(ctx context.Context, req *connect.Request[nydusv1.CreateChatroomRequest]) (*connect.Response[nydusv1.Chatroom], error) {
	id, err := randomHex(8)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	now := time.Now().Unix()
	room := &nydusv1.Chatroom{
		ChatroomId:  id,
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		Members:     req.Msg.Members,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateChatroom(room); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, m := range req.Msg.Members {
		if m.JoinedAt == 0 {
			m.JoinedAt = now
		}
		_ = h.store.AddMember(id, m)
	}
	return connect.NewResponse(room), nil
}

func (h *Handler) GetChatroom(ctx context.Context, req *connect.Request[nydusv1.GetChatroomRequest]) (*connect.Response[nydusv1.Chatroom], error) {
	room, err := h.store.GetChatroom(req.Msg.ChatroomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(room), nil
}

func (h *Handler) UpdateChatroom(ctx context.Context, req *connect.Request[nydusv1.UpdateChatroomRequest]) (*connect.Response[nydusv1.Chatroom], error) {
	room, err := h.store.UpdateChatroom(req.Msg.ChatroomId, req.Msg.Name, req.Msg.Description)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(room), nil
}

func (h *Handler) DeleteChatroom(ctx context.Context, req *connect.Request[nydusv1.DeleteChatroomRequest]) (*connect.Response[nydusv1.Empty], error) {
	if err := h.store.DeleteChatroom(req.Msg.ChatroomId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&nydusv1.Empty{}), nil
}

func (h *Handler) ListChatrooms(ctx context.Context, req *connect.Request[nydusv1.ListChatroomsRequest]) (*connect.Response[nydusv1.ListChatroomsResponse], error) {
	rooms, total, err := h.store.ListChatrooms(req.Msg.MemberId, req.Msg.Limit, req.Msg.Offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&nydusv1.ListChatroomsResponse{Chatrooms: rooms, Total: total}), nil
}

// ── Members ──────────────────────────────────────────────────────────────────

func (h *Handler) AddMember(ctx context.Context, req *connect.Request[nydusv1.AddMemberRequest]) (*connect.Response[nydusv1.Member], error) {
	m := req.Msg.Member
	if m == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if m.JoinedAt == 0 {
		m.JoinedAt = time.Now().Unix()
	}
	if err := h.store.AddMember(req.Msg.ChatroomId, m); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(m), nil
}

func (h *Handler) RemoveMember(ctx context.Context, req *connect.Request[nydusv1.RemoveMemberRequest]) (*connect.Response[nydusv1.Empty], error) {
	if err := h.store.RemoveMember(req.Msg.ChatroomId, req.Msg.MemberId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&nydusv1.Empty{}), nil
}

func (h *Handler) ListMembers(ctx context.Context, req *connect.Request[nydusv1.ListMembersRequest]) (*connect.Response[nydusv1.ListMembersResponse], error) {
	members, err := h.store.ListMembers(req.Msg.ChatroomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&nydusv1.ListMembersResponse{Members: members}), nil
}

func (h *Handler) UpdateMemberRole(ctx context.Context, req *connect.Request[nydusv1.UpdateMemberRoleRequest]) (*connect.Response[nydusv1.Member], error) {
	m, err := h.store.UpdateMemberRole(req.Msg.ChatroomId, req.Msg.MemberId, req.Msg.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(m), nil
}

// ── Messages ─────────────────────────────────────────────────────────────────

func (h *Handler) SendMessage(ctx context.Context, req *connect.Request[nydusv1.SendMessageRequest]) (*connect.Response[nydusv1.Message], error) {
	id, err := randomHex(8)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	msg := &nydusv1.Message{
		MessageId:  id,
		ChatroomId: req.Msg.ChatroomId,
		SenderId:   req.Msg.SenderId,
		SenderType: req.Msg.SenderType,
		Content:    req.Msg.Content,
		Metadata:   req.Msg.Metadata,
		CreatedAt:  time.Now().Unix(),
	}
	if err := h.store.SaveMessage(msg); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	h.messages.publish(msg)
	return connect.NewResponse(msg), nil
}

func (h *Handler) GetMessageHistory(ctx context.Context, req *connect.Request[nydusv1.GetMessageHistoryRequest]) (*connect.Response[nydusv1.GetMessageHistoryResponse], error) {
	msgs, total, err := h.store.GetMessageHistory(req.Msg.ChatroomId, req.Msg.Limit, req.Msg.Offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&nydusv1.GetMessageHistoryResponse{Messages: msgs, Total: total}), nil
}

func (h *Handler) SubscribeMessages(ctx context.Context, req *connect.Request[nydusv1.SubscribeMessagesRequest], stream *connect.ServerStream[nydusv1.Message]) error {
	instanceID := req.Msg.InstanceId
	if instanceID == "" {
		var err error
		if instanceID, err = randomHex(8); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	ch := h.messages.subscribe(req.Msg.ChatroomId, instanceID)
	defer h.messages.unsubscribe(req.Msg.ChatroomId, instanceID)

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
