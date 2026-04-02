package store

import (
	"database/sql"
	"time"
)

func (s *Store) migrateChatroom() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS chatrooms (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chatroom_members (
			id TEXT PRIMARY KEY,
			chatroom_id TEXT NOT NULL REFERENCES chatrooms(id) ON DELETE CASCADE,
			member_id TEXT NOT NULL,
			member_type TEXT NOT NULL,
			role TEXT NOT NULL,
			joined_at INTEGER NOT NULL,
			UNIQUE(chatroom_id, member_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chatroom_members_chatroom_id ON chatroom_members(chatroom_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chatroom_members_member_id ON chatroom_members(member_id)`,
		`CREATE TABLE IF NOT EXISTS chatroom_messages (
			id TEXT PRIMARY KEY,
			chatroom_id TEXT NOT NULL REFERENCES chatrooms(id) ON DELETE CASCADE,
			sender_id TEXT NOT NULL,
			sender_type TEXT NOT NULL,
			content TEXT NOT NULL,
			reply_to TEXT,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chatroom_messages_chatroom_id ON chatroom_messages(chatroom_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chatroom_messages_created_at ON chatroom_messages(created_at)`,
		`CREATE TABLE IF NOT EXISTS chatroom_sessions (
			id TEXT PRIMARY KEY,
			chatroom_id TEXT NOT NULL REFERENCES chatrooms(id) ON DELETE CASCADE,
			instance_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			external_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			UNIQUE(chatroom_id, instance_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chatroom_sessions_chatroom_id ON chatroom_sessions(chatroom_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chatroom_sessions_instance_id ON chatroom_sessions(instance_id)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateChatroom(chatroom *Chatroom) error {
	_, err := s.db.Exec(`
		INSERT INTO chatrooms (id, name, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, chatroom.ID, chatroom.Name, chatroom.CreatedBy, chatroom.CreatedAt, chatroom.UpdatedAt)
	return err
}

func (s *Store) GetChatroom(id string) (*Chatroom, error) {
	chatroom := &Chatroom{}
	err := s.db.QueryRow(`
		SELECT id, name, created_by, created_at, updated_at
		FROM chatrooms WHERE id = ?
	`, id).Scan(&chatroom.ID, &chatroom.Name, &chatroom.CreatedBy, &chatroom.CreatedAt, &chatroom.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return chatroom, err
}

func (s *Store) UpdateChatroom(id string, name string) error {
	_, err := s.db.Exec(`
		UPDATE chatrooms SET name = ?, updated_at = ? WHERE id = ?
	`, name, time.Now().Unix(), id)
	return err
}

func (s *Store) DeleteChatroom(id string) error {
	_, err := s.db.Exec(`DELETE FROM chatrooms WHERE id = ?`, id)
	return err
}

func (s *Store) ListChatrooms(memberID, memberType *string, limit, offset int) ([]*Chatroom, int, error) {
	var chatrooms []*Chatroom
	var total int

	if memberID != nil {
		err := s.db.QueryRow(`
			SELECT COUNT(DISTINCT c.id) FROM chatrooms c
			JOIN chatroom_members m ON c.id = m.chatroom_id
			WHERE m.member_id = ? AND (m.member_type = ? OR ? IS NULL)
		`, *memberID, memberType, memberType).Scan(&total)
		if err != nil {
			return nil, 0, err
		}

		rows, err := s.db.Query(`
			SELECT DISTINCT c.id, c.name, c.created_by, c.created_at, c.updated_at
			FROM chatrooms c
			JOIN chatroom_members m ON c.id = m.chatroom_id
			WHERE m.member_id = ? AND (m.member_type = ? OR ? IS NULL)
			ORDER BY c.updated_at DESC LIMIT ? OFFSET ?
		`, *memberID, memberType, memberType, limit, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()

		for rows.Next() {
			c := &Chatroom{}
			if err := rows.Scan(&c.ID, &c.Name, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
				return nil, 0, err
			}
			chatrooms = append(chatrooms, c)
		}
	} else {
		err := s.db.QueryRow(`SELECT COUNT(*) FROM chatrooms`).Scan(&total)
		if err != nil {
			return nil, 0, err
		}

		rows, err := s.db.Query(`
			SELECT id, name, created_by, created_at, updated_at
			FROM chatrooms ORDER BY updated_at DESC LIMIT ? OFFSET ?
		`, limit, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()

		for rows.Next() {
			c := &Chatroom{}
			if err := rows.Scan(&c.ID, &c.Name, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
				return nil, 0, err
			}
			chatrooms = append(chatrooms, c)
		}
	}

	return chatrooms, total, nil
}

func (s *Store) AddMember(member *Member) error {
	_, err := s.db.Exec(`
		INSERT INTO chatroom_members (id, chatroom_id, member_id, member_type, role, joined_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, member.ID, member.ChatroomID, member.MemberID, member.MemberType, member.Role, member.JoinedAt)
	return err
}

func (s *Store) RemoveMember(chatroomID, memberID string) error {
	_, err := s.db.Exec(`
		DELETE FROM chatroom_members WHERE chatroom_id = ? AND member_id = ?
	`, chatroomID, memberID)
	return err
}

func (s *Store) ListMembers(chatroomID string) ([]*Member, error) {
	rows, err := s.db.Query(`
		SELECT id, chatroom_id, member_id, member_type, role, joined_at
		FROM chatroom_members WHERE chatroom_id = ? ORDER BY joined_at ASC
	`, chatroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*Member
	for rows.Next() {
		m := &Member{}
		if err := rows.Scan(&m.ID, &m.ChatroomID, &m.MemberID, &m.MemberType, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func (s *Store) GetMember(chatroomID, memberID string) (*Member, error) {
	m := &Member{}
	err := s.db.QueryRow(`
		SELECT id, chatroom_id, member_id, member_type, role, joined_at
		FROM chatroom_members WHERE chatroom_id = ? AND member_id = ?
	`, chatroomID, memberID).Scan(&m.ID, &m.ChatroomID, &m.MemberID, &m.MemberType, &m.Role, &m.JoinedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (s *Store) UpdateMemberRole(chatroomID, memberID, role string) error {
	_, err := s.db.Exec(`
		UPDATE chatroom_members SET role = ? WHERE chatroom_id = ? AND member_id = ?
	`, role, chatroomID, memberID)
	return err
}

func (s *Store) CreateMessage(message *Message) error {
	_, err := s.db.Exec(`
		INSERT INTO chatroom_messages (id, chatroom_id, sender_id, sender_type, content, reply_to, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, message.ID, message.ChatroomID, message.SenderID, message.SenderType, message.Content, message.ReplyTo, message.CreatedAt)
	return err
}

func (s *Store) GetMessageHistory(chatroomID string, beforeID *string, limit int) ([]*Message, bool, error) {
	var messages []*Message
	var hasMore bool

	var rows *sql.Rows
	var err error

	if beforeID != nil {
		var beforeCreatedAt int64
		err := s.db.QueryRow(`
			SELECT created_at FROM chatroom_messages WHERE id = ?
		`, *beforeID).Scan(&beforeCreatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, false, nil
			}
			return nil, false, err
		}

		rows, err = s.db.Query(`
			SELECT id, chatroom_id, sender_id, sender_type, content, reply_to, created_at
			FROM chatroom_messages
			WHERE chatroom_id = ? AND created_at < ?
			ORDER BY created_at DESC LIMIT ?
		`, chatroomID, beforeCreatedAt, limit+1)
	} else {
		rows, err = s.db.Query(`
			SELECT id, chatroom_id, sender_id, sender_type, content, reply_to, created_at
			FROM chatroom_messages
			WHERE chatroom_id = ?
			ORDER BY created_at DESC LIMIT ?
		`, chatroomID, limit+1)
	}

	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	for rows.Next() {
		m := &Message{}
		if err := rows.Scan(&m.ID, &m.ChatroomID, &m.SenderID, &m.SenderType, &m.Content, &m.ReplyTo, &m.CreatedAt); err != nil {
			return nil, false, err
		}
		messages = append(messages, m)
	}

	if len(messages) > limit {
		hasMore = true
		messages = messages[:limit]
	}

	return messages, hasMore, nil
}

func (s *Store) CreateChatroomSession(session *ChatroomSession) error {
	_, err := s.db.Exec(`
		INSERT INTO chatroom_sessions (id, chatroom_id, instance_id, session_id, external_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, session.ID, session.ChatroomID, session.InstanceID, session.SessionID, session.ExternalID, session.CreatedAt)
	return err
}

func (s *Store) GetChatroomSessions(chatroomID string) ([]*ChatroomSession, error) {
	rows, err := s.db.Query(`
		SELECT id, chatroom_id, instance_id, session_id, external_id, created_at
		FROM chatroom_sessions WHERE chatroom_id = ?
	`, chatroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*ChatroomSession
	for rows.Next() {
		s := &ChatroomSession{}
		if err := rows.Scan(&s.ID, &s.ChatroomID, &s.InstanceID, &s.SessionID, &s.ExternalID, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (s *Store) GetChatroomSession(chatroomID, instanceID string) (*ChatroomSession, error) {
	session := &ChatroomSession{}
	err := s.db.QueryRow(`
		SELECT id, chatroom_id, instance_id, session_id, external_id, created_at
		FROM chatroom_sessions WHERE chatroom_id = ? AND instance_id = ?
	`, chatroomID, instanceID).Scan(&session.ID, &session.ChatroomID, &session.InstanceID, &session.SessionID, &session.ExternalID, &session.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return session, err
}

func (s *Store) UpdateChatroomSession(chatroomID, instanceID, sessionID string) error {
	_, err := s.db.Exec(`
		UPDATE chatroom_sessions SET session_id = ? WHERE chatroom_id = ? AND instance_id = ?
	`, sessionID, chatroomID, instanceID)
	return err
}

func (s *Store) DeleteChatroomSession(chatroomID, instanceID string) error {
	_, err := s.db.Exec(`
		DELETE FROM chatroom_sessions WHERE chatroom_id = ? AND instance_id = ?
	`, chatroomID, instanceID)
	return err
}

func (s *Store) GetInstanceMembers(chatroomID string) ([]*Member, error) {
	rows, err := s.db.Query(`
		SELECT id, chatroom_id, member_id, member_type, role, joined_at
		FROM chatroom_members WHERE chatroom_id = ? AND member_type = 'instance'
	`, chatroomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*Member
	for rows.Next() {
		m := &Member{}
		if err := rows.Scan(&m.ID, &m.ChatroomID, &m.MemberID, &m.MemberType, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}
