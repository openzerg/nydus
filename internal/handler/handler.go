package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"connectrpc.com/connect"
	nydusv1 "github.com/openzerg/common/gen/nydus/v1"
	nydusv1connect "github.com/openzerg/common/gen/nydus/v1/nydusv1connect"

	"github.com/openzerg/nydus/internal/config"
	"github.com/openzerg/nydus/internal/store"
)

var _ nydusv1connect.NydusServiceHandler = (*Handler)(nil)

type Handler struct {
	store    *store.Store
	events   *eventFanout
	messages *messageFanout
	cfg      *config.Config
}

func New(st *store.Store, cfg *config.Config) *Handler {
	return &Handler{
		store:    st,
		events:   newEventFanout(),
		messages: newMessageFanout(),
		cfg:      cfg,
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
	if _, err := h.store.GetChatroom(req.Msg.ChatroomId); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("chatroom %s not found", req.Msg.ChatroomId))
	}
	id, err := randomHex(8)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	msg := &nydusv1.Message{
		MessageId:   id,
		ChatroomId:  req.Msg.ChatroomId,
		SenderId:    req.Msg.SenderId,
		SenderType:  req.Msg.SenderType,
		Content:     req.Msg.Content,
		Metadata:    req.Msg.Metadata,
		CreatedAt:   time.Now().Unix(),
		MessageType: req.Msg.MessageType,
		ReplyToId:   req.Msg.ReplyToId,
		Attachments: req.Msg.Attachments,
		Mentions:    req.Msg.Mentions,
	}
	if err := h.store.SaveMessage(msg); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	h.messages.publish(msg)
	go h.notifyMentionedInstances(msg)
	return connect.NewResponse(msg), nil
}

func (h *Handler) notifyMentionedInstances(msg *nydusv1.Message) {
	if len(msg.Mentions) == 0 {
		return
	}

	members, err := h.store.ListMembers(msg.ChatroomId)
	if err != nil {
		log.Printf("[notify] failed to list members for chatroom %s: %v", msg.ChatroomId, err)
		return
	}

	memberMap := make(map[string]*nydusv1.Member)
	for i := range members {
		memberMap[members[i].MemberId] = members[i]
	}

	for _, mention := range msg.Mentions {
		member, ok := memberMap[mention.MemberId]
		if !ok || member.MemberType != "instance" {
			continue
		}

		agentURL := h.resolveAgentURL(member.MemberId)
		if agentURL == "" {
			log.Printf("[notify] could not resolve agent URL for %s", member.MemberId)
			continue
		}

		mentionsData := make([]map[string]string, 0, len(msg.Mentions))
		for _, m := range msg.Mentions {
			mentionsData = append(mentionsData, map[string]string{
				"member_id":   m.MemberId,
				"member_name": m.MemberName,
			})
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"chatroom_id": msg.ChatroomId,
			"message_id":  msg.MessageId,
			"sender_id":   msg.SenderId,
			"sender_type": msg.SenderType,
			"content":     msg.Content,
			"mentions":    mentionsData,
		})

		notifyReq := map[string]interface{}{
			"notify_type": "mention",
			"source_id":   msg.ChatroomId,
			"content":     fmt.Sprintf("[%s:%s] %s", msg.SenderType, msg.SenderId, msg.Content),
			"payload":     string(payload),
			"metadata": map[string]string{
				"chatroom_id": msg.ChatroomId,
				"message_id":  msg.MessageId,
				"sender_id":   msg.SenderId,
				"sender_type": msg.SenderType,
			},
		}

		go h.pushNotify(agentURL, notifyReq)
	}
}

func (h *Handler) resolveAgentURL(instanceID string) string {
	if h.cfg.CerebrateURL == "" {
		return ""
	}

	body, _ := json.Marshal(map[string]interface{}{
		"instance_id": instanceID,
	})

	url := h.cfg.CerebrateURL + "/cerebrate.v1.CerebrateService/GetInstance"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return ""
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if h.cfg.AdminToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+h.cfg.AdminToken)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Printf("[notify] failed to query cerebrate for %s: %v", instanceID, err)
		return ""
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	instance, ok := result["instance"].(map[string]interface{})
	if !ok {
		return ""
	}
	urlStr, _ := instance["url"].(string)
	return urlStr
}

func (h *Handler) pushNotify(agentURL string, req map[string]interface{}) {
	endpoint := agentURL + "/agent.v1.AgentService/ReceiveNotify"
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("[notify] failed to create request: %v", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Printf("[notify] failed to push to %s: %v", endpoint, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[notify] push to %s returned status %d", endpoint, resp.StatusCode)
	}
}

func (h *Handler) EditMessage(ctx context.Context, req *connect.Request[nydusv1.EditMessageRequest]) (*connect.Response[nydusv1.Message], error) {
	msgs, _, err := h.store.GetMessageHistory(req.Msg.ChatroomId, 1000, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var msg *nydusv1.Message
	for _, m := range msgs {
		if m.MessageId == req.Msg.MessageId {
			msg = m
			break
		}
	}
	if msg == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("message not found"))
	}
	msg.Content = req.Msg.Content
	msg.UpdatedAt = time.Now().Unix()
	if err := h.store.UpdateMessage(msg); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	h.messages.publish(msg)
	return connect.NewResponse(msg), nil
}

func (h *Handler) DeleteMessage(ctx context.Context, req *connect.Request[nydusv1.DeleteMessageRequest]) (*connect.Response[nydusv1.Empty], error) {
	if err := h.store.DeleteMessage(req.Msg.ChatroomId, req.Msg.MessageId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&nydusv1.Empty{}), nil
}

func (h *Handler) ReactMessage(ctx context.Context, req *connect.Request[nydusv1.ReactMessageRequest]) (*connect.Response[nydusv1.Message], error) {
	var err error
	if req.Msg.Remove {
		err = h.store.RemoveReaction(req.Msg.ChatroomId, req.Msg.MessageId, req.Msg.Emoji, req.Msg.UserId)
	} else {
		err = h.store.AddReaction(req.Msg.ChatroomId, req.Msg.MessageId, req.Msg.Emoji, req.Msg.UserId)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	msgs, _, err := h.store.GetMessageHistory(req.Msg.ChatroomId, 1000, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, m := range msgs {
		if m.MessageId == req.Msg.MessageId {
			h.messages.publish(m)
			return connect.NewResponse(m), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("message not found"))
}

func (h *Handler) Typing(ctx context.Context, req *connect.Request[nydusv1.TypingRequest]) (*connect.Response[nydusv1.Empty], error) {
	h.messages.publishTyping(req.Msg.ChatroomId, &nydusv1.TypingEvent{
		ChatroomId: req.Msg.ChatroomId,
		UserId:     req.Msg.UserId,
		UserName:   req.Msg.UserName,
		Timestamp:  time.Now().Unix(),
	})
	return connect.NewResponse(&nydusv1.Empty{}), nil
}

func (h *Handler) SubscribeTyping(ctx context.Context, req *connect.Request[nydusv1.TypingRequest], stream *connect.ServerStream[nydusv1.TypingEvent]) error {
	ch := h.messages.subscribeTyping(req.Msg.ChatroomId)
	defer h.messages.unsubscribeTyping(req.Msg.ChatroomId)

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
