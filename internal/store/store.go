package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type Event struct {
	EventID          string
	EventType        string
	SourceInstanceID string
	Timestamp        int64
	Data             map[string]string
}

type Subscriber struct {
	InstanceID   string
	IP           string
	EventTypes   []string
	SubscribedAt int64
}

type SubscriberConnection struct {
	InstanceID string
	IP         string
	EventTypes []string
	Channel    chan *Event
	Done       chan struct{}
}

type MessageSubscriber struct {
	InstanceID string
	ChatroomID string
	Channel    chan *Message
	Done       chan struct{}
}

type SubscriptionManager struct {
	mu          sync.RWMutex
	connections map[string]*SubscriberConnection
}

type MessageSubscriptionManager struct {
	mu          sync.RWMutex
	connections map[string]*MessageSubscriber // key: chatroom_id:instance_id
}

func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		connections: make(map[string]*SubscriberConnection),
	}
}

func NewMessageSubscriptionManager() *MessageSubscriptionManager {
	return &MessageSubscriptionManager{
		connections: make(map[string]*MessageSubscriber),
	}
}

func (sm *SubscriptionManager) Add(instanceID, ip string, eventTypes []string) *SubscriberConnection {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if existing, ok := sm.connections[instanceID]; ok {
		close(existing.Done)
		delete(sm.connections, instanceID)
	}

	conn := &SubscriberConnection{
		InstanceID: instanceID,
		IP:         ip,
		EventTypes: eventTypes,
		Channel:    make(chan *Event, 100),
		Done:       make(chan struct{}),
	}
	sm.connections[instanceID] = conn
	return conn
}

func (sm *SubscriptionManager) Remove(instanceID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if conn, ok := sm.connections[instanceID]; ok {
		close(conn.Done)
		delete(sm.connections, instanceID)
	}
}

func (sm *SubscriptionManager) Broadcast(event *Event) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for instanceID, conn := range sm.connections {
		if instanceID == event.SourceInstanceID {
			continue
		}

		if len(conn.EventTypes) == 0 {
			select {
			case conn.Channel <- event:
			default:
			}
			continue
		}

		for _, et := range conn.EventTypes {
			if et == event.EventType {
				select {
				case conn.Channel <- event:
				default:
				}
				break
			}
		}
	}
}

func (sm *SubscriptionManager) List(eventType string) []*Subscriber {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var subscribers []*Subscriber
	for _, conn := range sm.connections {
		if eventType != "" {
			found := false
			for _, et := range conn.EventTypes {
				if et == eventType {
					found = true
					break
				}
			}
			if !found && len(conn.EventTypes) > 0 {
				continue
			}
		}
		subscribers = append(subscribers, &Subscriber{
			InstanceID:   conn.InstanceID,
			IP:           conn.IP,
			EventTypes:   conn.EventTypes,
			SubscribedAt: time.Now().Unix(),
		})
	}
	return subscribers
}

func (sm *SubscriptionManager) Get(instanceID string) (*SubscriberConnection, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	conn, ok := sm.connections[instanceID]
	return conn, ok
}

func (msm *MessageSubscriptionManager) Add(chatroomID, instanceID string) *MessageSubscriber {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	key := chatroomID + ":" + instanceID

	if existing, ok := msm.connections[key]; ok {
		close(existing.Done)
		delete(msm.connections, key)
	}

	sub := &MessageSubscriber{
		InstanceID: instanceID,
		ChatroomID: chatroomID,
		Channel:    make(chan *Message, 100),
		Done:       make(chan struct{}),
	}
	msm.connections[key] = sub
	return sub
}

func (msm *MessageSubscriptionManager) Remove(chatroomID, instanceID string) {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	key := chatroomID + ":" + instanceID
	if sub, ok := msm.connections[key]; ok {
		close(sub.Done)
		delete(msm.connections, key)
	}
}

func (msm *MessageSubscriptionManager) Broadcast(chatroomID string, message *Message) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	for _, sub := range msm.connections {
		if sub.ChatroomID != chatroomID {
			continue
		}

		if sub.InstanceID == message.SenderID {
			continue
		}

		select {
		case sub.Channel <- message:
		default:
		}
	}
}

func (msm *MessageSubscriptionManager) Get(chatroomID, instanceID string) (*MessageSubscriber, bool) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()
	key := chatroomID + ":" + instanceID
	sub, ok := msm.connections[key]
	return sub, ok
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return s, nil
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS events (
			event_id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL,
			source_instance_id TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			data JSON
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_events_source ON events(source_instance_id)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}

	return s.migrateChatroom()
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SaveEvent(event *Event) error {
	dataJSON, _ := json.Marshal(event.Data)
	_, err := s.db.Exec(`
		INSERT INTO events (event_id, event_type, source_instance_id, timestamp, data)
		VALUES (?, ?, ?, ?, ?)
	`, event.EventID, event.EventType, event.SourceInstanceID, event.Timestamp, dataJSON)
	return err
}

func (s *Store) GetEventHistory(instanceID, eventType *string, startTime, endTime int64, limit int) ([]*Event, error) {
	query := `SELECT event_id, event_type, source_instance_id, timestamp, data FROM events WHERE timestamp >= ? AND timestamp <= ?`
	args := []interface{}{startTime, endTime}

	if instanceID != nil {
		query += " AND source_instance_id = ?"
		args = append(args, *instanceID)
	}
	if eventType != nil {
		query += " AND event_type = ?"
		args = append(args, *eventType)
	}
	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var dataJSON []byte
		event := &Event{}
		err := rows.Scan(&event.EventID, &event.EventType, &event.SourceInstanceID, &event.Timestamp, &dataJSON)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(dataJSON, &event.Data)
		events = append(events, event)
	}
	return events, nil
}
