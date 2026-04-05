package store

import (
	"encoding/json"
	"time"

	nydusv1 "github.com/openzerg/common/gen/nydus/v1"
)

// ── Chatrooms ────────────────────────────────────────────────────────────────

func (s *Store) CreateChatroom(c *nydusv1.Chatroom) error {
	_, err := s.db.Exec(`
		INSERT INTO chatrooms (chatroom_id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		c.ChatroomId, c.Name, c.Description, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

func (s *Store) GetChatroom(id string) (*nydusv1.Chatroom, error) {
	var c nydusv1.Chatroom
	err := s.db.QueryRow(`SELECT chatroom_id, name, description, created_at, updated_at FROM chatrooms WHERE chatroom_id = ?`, id).
		Scan(&c.ChatroomId, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	members, err := s.ListMembers(id)
	if err != nil {
		return nil, err
	}
	c.Members = members
	return &c, nil
}

func (s *Store) UpdateChatroom(id, name, description string) (*nydusv1.Chatroom, error) {
	now := time.Now().Unix()
	_, err := s.db.Exec(`UPDATE chatrooms SET name=?, description=?, updated_at=? WHERE chatroom_id=?`,
		name, description, now, id)
	if err != nil {
		return nil, err
	}
	return s.GetChatroom(id)
}

func (s *Store) DeleteChatroom(id string) error {
	_, err := s.db.Exec(`DELETE FROM chatrooms WHERE chatroom_id = ?`, id)
	return err
}

func (s *Store) ListChatrooms(memberID string, limit, offset int32) ([]*nydusv1.Chatroom, int32, error) {
	var q, cq string
	var args []any

	if memberID != "" {
		q = `SELECT c.chatroom_id, c.name, c.description, c.created_at, c.updated_at
			 FROM chatrooms c JOIN members m ON c.chatroom_id = m.chatroom_id
			 WHERE m.member_id = ? ORDER BY c.created_at DESC LIMIT ? OFFSET ?`
		cq = `SELECT COUNT(*) FROM chatrooms c JOIN members m ON c.chatroom_id = m.chatroom_id WHERE m.member_id = ?`
		args = []any{memberID, limit, offset}
	} else {
		q = `SELECT chatroom_id, name, description, created_at, updated_at FROM chatrooms ORDER BY created_at DESC LIMIT ? OFFSET ?`
		cq = `SELECT COUNT(*) FROM chatrooms`
		args = []any{limit, offset}
	}

	var total int32
	cqArgs := args[:len(args)-2]
	if memberID == "" {
		cqArgs = nil
	}
	if err := s.db.QueryRow(cq, cqArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
		args[len(args)-2] = limit
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var rooms []*nydusv1.Chatroom
	for rows.Next() {
		var c nydusv1.Chatroom
		if err := rows.Scan(&c.ChatroomId, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, err
		}
		rooms = append(rooms, &c)
	}
	return rooms, total, rows.Err()
}

// ── Members ──────────────────────────────────────────────────────────────────

func (s *Store) AddMember(chatroomID string, m *nydusv1.Member) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO members (chatroom_id, member_id, member_type, role, joined_at)
		VALUES (?, ?, ?, ?, ?)`,
		chatroomID, m.MemberId, m.MemberType, m.Role, m.JoinedAt,
	)
	return err
}

func (s *Store) RemoveMember(chatroomID, memberID string) error {
	_, err := s.db.Exec(`DELETE FROM members WHERE chatroom_id=? AND member_id=?`, chatroomID, memberID)
	return err
}

func (s *Store) ListMembers(chatroomID string) ([]*nydusv1.Member, error) {
	rows, err := s.db.Query(`SELECT member_id, member_type, role, joined_at FROM members WHERE chatroom_id=?`, chatroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []*nydusv1.Member
	for rows.Next() {
		var m nydusv1.Member
		if err := rows.Scan(&m.MemberId, &m.MemberType, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, &m)
	}
	return members, rows.Err()
}

func (s *Store) UpdateMemberRole(chatroomID, memberID, role string) (*nydusv1.Member, error) {
	_, err := s.db.Exec(`UPDATE members SET role=? WHERE chatroom_id=? AND member_id=?`, role, chatroomID, memberID)
	if err != nil {
		return nil, err
	}
	var m nydusv1.Member
	err = s.db.QueryRow(`SELECT member_id, member_type, role, joined_at FROM members WHERE chatroom_id=? AND member_id=?`, chatroomID, memberID).
		Scan(&m.MemberId, &m.MemberType, &m.Role, &m.JoinedAt)
	return &m, err
}

// ── Messages ─────────────────────────────────────────────────────────────────

func (s *Store) SaveMessage(msg *nydusv1.Message) error {
	meta, _ := json.Marshal(msg.Metadata)
	_, err := s.db.Exec(`
		INSERT INTO messages (message_id, chatroom_id, sender_id, sender_type, content, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.MessageId, msg.ChatroomId, msg.SenderId, msg.SenderType, msg.Content, string(meta), msg.CreatedAt,
	)
	return err
}

func (s *Store) GetMessageHistory(chatroomID string, limit, offset int32) ([]*nydusv1.Message, int32, error) {
	var total int32
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE chatroom_id=?`, chatroomID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT message_id, chatroom_id, sender_id, sender_type, content, metadata, created_at
		FROM messages WHERE chatroom_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		chatroomID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var msgs []*nydusv1.Message
	for rows.Next() {
		var m nydusv1.Message
		var metaJSON string
		if err := rows.Scan(&m.MessageId, &m.ChatroomId, &m.SenderId, &m.SenderType, &m.Content, &metaJSON, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal([]byte(metaJSON), &m.Metadata)
		msgs = append(msgs, &m)
	}
	return msgs, total, rows.Err()
}
