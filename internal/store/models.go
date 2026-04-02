package store

type Chatroom struct {
	ID        string
	Name      string
	CreatedBy string
	CreatedAt int64
	UpdatedAt int64
}

type Member struct {
	ID         string
	ChatroomID string
	MemberID   string
	MemberType string
	Role       string
	JoinedAt   int64
}

type Message struct {
	ID         string
	ChatroomID string
	SenderID   string
	SenderType string
	Content    string
	ReplyTo    *string
	CreatedAt  int64
}

type ChatroomSession struct {
	ID         string
	ChatroomID string
	InstanceID string
	SessionID  string
	ExternalID string
	CreatedAt  int64
}
