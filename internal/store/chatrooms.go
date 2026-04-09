package store

import (
	"context"
	"encoding/json"
	"time"

	nydusv1 "github.com/openzerg/common/gen/nydus/v1"
	"github.com/uptrace/bun"
)

// ── Models ────────────────────────────────────────────────────────────────────

type Chatroom struct {
	bun.BaseModel `bun:"table:chatrooms,alias:c"`

	ChatroomID  string `bun:"chatroom_id,pk"`
	Name        string `bun:"name,notnull"`
	Description string `bun:"description,default:''"`
	CreatedAt   int64  `bun:"created_at,notnull"`
	UpdatedAt   int64  `bun:"updated_at,notnull"`
}

type Member struct {
	bun.BaseModel `bun:"table:members,alias:m"`

	ChatroomID string `bun:"chatroom_id,notnull"`
	MemberID   string `bun:"member_id,notnull"`
	MemberType string `bun:"member_type,default:'user'"`
	Role       string `bun:"role,default:'member'"`
	JoinedAt   int64  `bun:"joined_at,notnull"`
}

type Message struct {
	bun.BaseModel `bun:"table:messages,alias:msg"`

	MessageID   string `bun:"message_id,pk"`
	ChatroomID  string `bun:"chatroom_id,notnull"`
	SenderID    string `bun:"sender_id,notnull"`
	SenderType  string `bun:"sender_type,default:'user'"`
	Content     string `bun:"content,default:''"`
	Metadata    string `bun:"metadata,type:text,default:'{}'"`
	CreatedAt   int64  `bun:"created_at,notnull"`
	MessageType string `bun:"message_type,default:'text'"`
	UpdatedAt   int64  `bun:"updated_at,default:0"`
	Deleted     int    `bun:"deleted,default:0"`
	ReplyToID   string `bun:"reply_to_id,default:''"`
	Attachments string `bun:"attachments,type:text,default:'[]'"`
	Mentions    string `bun:"mentions,type:text,default:'[]'"`
}

type Reaction struct {
	bun.BaseModel `bun:"table:reactions,alias:r"`

	MessageID string `bun:"message_id,notnull"`
	Emoji     string `bun:"emoji,notnull"`
	UserID    string `bun:"user_id,notnull"`
	CreatedAt int64  `bun:"created_at,notnull"`
}

// ── Chatrooms ─────────────────────────────────────────────────────────────────

func (s *Store) CreateChatroom(c *nydusv1.Chatroom) error {
	row := &Chatroom{
		ChatroomID:  c.ChatroomId,
		Name:        c.Name,
		Description: c.Description,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
	_, err := s.db.NewInsert().Model(row).Exec(context.Background())
	return err
}

func (s *Store) GetChatroom(id string) (*nydusv1.Chatroom, error) {
	ctx := context.Background()
	var row Chatroom
	if err := s.db.NewSelect().Model(&row).Where("chatroom_id = ?", id).Scan(ctx); err != nil {
		return nil, err
	}
	c := &nydusv1.Chatroom{
		ChatroomId:  row.ChatroomID,
		Name:        row.Name,
		Description: row.Description,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	members, err := s.ListMembers(id)
	if err != nil {
		return nil, err
	}
	c.Members = members
	return c, nil
}

func (s *Store) UpdateChatroom(id, name, description string) (*nydusv1.Chatroom, error) {
	now := time.Now().Unix()
	_, err := s.db.NewUpdate().TableExpr("chatrooms").
		Set("name = ?", name).
		Set("description = ?", description).
		Set("updated_at = ?", now).
		Where("chatroom_id = ?", id).
		Exec(context.Background())
	if err != nil {
		return nil, err
	}
	return s.GetChatroom(id)
}

func (s *Store) DeleteChatroom(id string) error {
	_, err := s.db.NewDelete().TableExpr("chatrooms").
		Where("chatroom_id = ?", id).
		Exec(context.Background())
	return err
}

func (s *Store) ListChatrooms(memberID string, limit, offset int32) ([]*nydusv1.Chatroom, int32, error) {
	ctx := context.Background()
	if limit <= 0 {
		limit = 50
	}

	type chatroomRow struct {
		ChatroomID  string `bun:"chatroom_id"`
		Name        string `bun:"name"`
		Description string `bun:"description"`
		CreatedAt   int64  `bun:"created_at"`
		UpdatedAt   int64  `bun:"updated_at"`
	}

	var q *bun.SelectQuery
	if memberID != "" {
		q = s.db.NewSelect().
			TableExpr("chatrooms AS c").
			ColumnExpr("c.chatroom_id, c.name, c.description, c.created_at, c.updated_at").
			Join("JOIN members AS m ON c.chatroom_id = m.chatroom_id").
			Where("m.member_id = ?", memberID)
	} else {
		q = s.db.NewSelect().
			TableExpr("chatrooms AS c").
			ColumnExpr("c.chatroom_id, c.name, c.description, c.created_at, c.updated_at")
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	var rows []chatroomRow
	if err := q.OrderExpr("c.created_at DESC").Limit(int(limit)).Offset(int(offset)).Scan(ctx, &rows); err != nil {
		return nil, 0, err
	}

	rooms := make([]*nydusv1.Chatroom, len(rows))
	for i, r := range rows {
		rooms[i] = &nydusv1.Chatroom{
			ChatroomId: r.ChatroomID, Name: r.Name, Description: r.Description,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
	}
	return rooms, int32(total), nil
}

// ── Members ───────────────────────────────────────────────────────────────────

func (s *Store) AddMember(chatroomID string, m *nydusv1.Member) error {
	row := &Member{
		ChatroomID: chatroomID,
		MemberID:   m.MemberId,
		MemberType: m.MemberType,
		Role:       m.Role,
		JoinedAt:   m.JoinedAt,
	}
	_, err := s.db.NewInsert().Model(row).
		On("CONFLICT DO UPDATE SET member_type=EXCLUDED.member_type, role=EXCLUDED.role").
		Exec(context.Background())
	return err
}

func (s *Store) RemoveMember(chatroomID, memberID string) error {
	_, err := s.db.NewDelete().TableExpr("members").
		Where("chatroom_id = ? AND member_id = ?", chatroomID, memberID).
		Exec(context.Background())
	return err
}

func (s *Store) ListMembers(chatroomID string) ([]*nydusv1.Member, error) {
	ctx := context.Background()
	type memberRow struct {
		MemberID   string `bun:"member_id"`
		MemberType string `bun:"member_type"`
		Role       string `bun:"role"`
		JoinedAt   int64  `bun:"joined_at"`
	}
	var rows []memberRow
	err := s.db.NewSelect().TableExpr("members").
		ColumnExpr("member_id, member_type, role, joined_at").
		Where("chatroom_id = ?", chatroomID).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	members := make([]*nydusv1.Member, len(rows))
	for i, r := range rows {
		members[i] = &nydusv1.Member{MemberId: r.MemberID, MemberType: r.MemberType, Role: r.Role, JoinedAt: r.JoinedAt}
	}
	return members, nil
}

func (s *Store) UpdateMemberRole(chatroomID, memberID, role string) (*nydusv1.Member, error) {
	ctx := context.Background()
	_, err := s.db.NewUpdate().TableExpr("members").
		Set("role = ?", role).
		Where("chatroom_id = ? AND member_id = ?", chatroomID, memberID).
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	type memberRow struct {
		MemberID   string `bun:"member_id"`
		MemberType string `bun:"member_type"`
		Role       string `bun:"role"`
		JoinedAt   int64  `bun:"joined_at"`
	}
	var r memberRow
	err = s.db.NewSelect().TableExpr("members").
		ColumnExpr("member_id, member_type, role, joined_at").
		Where("chatroom_id = ? AND member_id = ?", chatroomID, memberID).
		Scan(ctx, &r)
	if err != nil {
		return nil, err
	}
	return &nydusv1.Member{MemberId: r.MemberID, MemberType: r.MemberType, Role: r.Role, JoinedAt: r.JoinedAt}, nil
}

// ── Messages ──────────────────────────────────────────────────────────────────

func (s *Store) SaveMessage(msg *nydusv1.Message) error {
	meta, _ := json.Marshal(msg.Metadata)
	attachments, _ := json.Marshal(msg.Attachments)
	mentions, _ := json.Marshal(msg.Mentions)
	msgType := "text"
	switch msg.MessageType {
	case nydusv1.MessageType_MESSAGE_TYPE_IMAGE:
		msgType = "image"
	case nydusv1.MessageType_MESSAGE_TYPE_FILE:
		msgType = "file"
	case nydusv1.MessageType_MESSAGE_TYPE_SYSTEM:
		msgType = "system"
	}
	row := &Message{
		MessageID:   msg.MessageId,
		ChatroomID:  msg.ChatroomId,
		SenderID:    msg.SenderId,
		SenderType:  msg.SenderType,
		Content:     msg.Content,
		Metadata:    string(meta),
		CreatedAt:   msg.CreatedAt,
		MessageType: msgType,
		UpdatedAt:   msg.UpdatedAt,
		ReplyToID:   msg.ReplyToId,
		Attachments: string(attachments),
		Mentions:    string(mentions),
	}
	_, err := s.db.NewInsert().Model(row).Exec(context.Background())
	return err
}

func (s *Store) UpdateMessage(msg *nydusv1.Message) error {
	attachments, _ := json.Marshal(msg.Attachments)
	mentions, _ := json.Marshal(msg.Mentions)
	_, err := s.db.NewUpdate().TableExpr("messages").
		Set("content = ?", msg.Content).
		Set("updated_at = ?", msg.UpdatedAt).
		Set("attachments = ?", string(attachments)).
		Set("mentions = ?", string(mentions)).
		Where("message_id = ? AND chatroom_id = ?", msg.MessageId, msg.ChatroomId).
		Exec(context.Background())
	return err
}

func (s *Store) DeleteMessage(chatroomID, messageID string) error {
	now := time.Now().Unix()
	_, err := s.db.NewUpdate().TableExpr("messages").
		Set("deleted = 1").
		Set("updated_at = ?", now).
		Where("message_id = ? AND chatroom_id = ?", messageID, chatroomID).
		Exec(context.Background())
	return err
}

func (s *Store) AddReaction(chatroomID, messageID, emoji, userID string) error {
	now := time.Now().Unix()
	row := &Reaction{
		MessageID: messageID,
		Emoji:     emoji,
		UserID:    userID,
		CreatedAt: now,
	}
	_, err := s.db.NewInsert().Model(row).
		On("CONFLICT DO NOTHING").
		Exec(context.Background())
	return err
}

func (s *Store) RemoveReaction(chatroomID, messageID, emoji, userID string) error {
	_, err := s.db.NewDelete().TableExpr("reactions").
		Where("message_id = ? AND emoji = ? AND user_id = ?", messageID, emoji, userID).
		Exec(context.Background())
	return err
}

func (s *Store) GetReactions(messageID string) ([]*nydusv1.Reaction, error) {
	ctx := context.Background()
	type reactionRow struct {
		Emoji  string `bun:"emoji"`
		UserID string `bun:"user_id"`
	}
	var rows []reactionRow
	err := s.db.NewSelect().TableExpr("reactions").
		ColumnExpr("emoji, user_id").
		Where("message_id = ?", messageID).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	// Group by emoji
	reactionMap := make(map[string]*nydusv1.Reaction)
	for _, r := range rows {
		if _, ok := reactionMap[r.Emoji]; !ok {
			reactionMap[r.Emoji] = &nydusv1.Reaction{Emoji: r.Emoji}
		}
		reactionMap[r.Emoji].UserIds = append(reactionMap[r.Emoji].UserIds, r.UserID)
	}

	reactions := make([]*nydusv1.Reaction, 0, len(reactionMap))
	for _, r := range reactionMap {
		reactions = append(reactions, r)
	}
	return reactions, nil
}

func (s *Store) GetMessageHistory(chatroomID string, limit, offset int32) ([]*nydusv1.Message, int32, error) {
	ctx := context.Background()
	if limit <= 0 {
		limit = 50
	}

	type msgRow struct {
		MessageID   string `bun:"message_id"`
		ChatroomID  string `bun:"chatroom_id"`
		SenderID    string `bun:"sender_id"`
		SenderType  string `bun:"sender_type"`
		Content     string `bun:"content"`
		Metadata    string `bun:"metadata"`
		CreatedAt   int64  `bun:"created_at"`
		MessageType string `bun:"message_type"`
		UpdatedAt   int64  `bun:"updated_at"`
		Deleted     int    `bun:"deleted"`
		ReplyToID   string `bun:"reply_to_id"`
		Attachments string `bun:"attachments"`
		Mentions    string `bun:"mentions"`
	}

	q := s.db.NewSelect().TableExpr("messages").
		ColumnExpr("message_id, chatroom_id, sender_id, sender_type, content, metadata, created_at, message_type, updated_at, deleted, reply_to_id, attachments, mentions").
		Where("chatroom_id = ?", chatroomID)

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	var rows []msgRow
	if err := q.OrderExpr("created_at DESC").Limit(int(limit)).Offset(int(offset)).Scan(ctx, &rows); err != nil {
		return nil, 0, err
	}

	msgs := make([]*nydusv1.Message, len(rows))
	for i, r := range rows {
		m := &nydusv1.Message{
			MessageId: r.MessageID, ChatroomId: r.ChatroomID,
			SenderId: r.SenderID, SenderType: r.SenderType,
			Content: r.Content, CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt, Deleted: r.Deleted == 1, ReplyToId: r.ReplyToID,
		}
		switch r.MessageType {
		case "image":
			m.MessageType = nydusv1.MessageType_MESSAGE_TYPE_IMAGE
		case "file":
			m.MessageType = nydusv1.MessageType_MESSAGE_TYPE_FILE
		case "system":
			m.MessageType = nydusv1.MessageType_MESSAGE_TYPE_SYSTEM
		default:
			m.MessageType = nydusv1.MessageType_MESSAGE_TYPE_TEXT
		}
		_ = json.Unmarshal([]byte(r.Metadata), &m.Metadata)
		_ = json.Unmarshal([]byte(r.Attachments), &m.Attachments)
		_ = json.Unmarshal([]byte(r.Mentions), &m.Mentions)

		// Load reactions
		reactions, _ := s.GetReactions(r.MessageID)
		m.Reactions = reactions

		msgs[i] = m
	}
	return msgs, int32(total), nil
}
