package store

import (
	"database/sql"
	"encoding/json"
	"time"

	nydusv1 "github.com/openzerg/common/gen/nydus/v1"
)

func (s *Store) SaveEvent(e *nydusv1.Event) error {
	data, _ := json.Marshal(e.Data)
	_, err := s.db.Exec(`
		INSERT INTO events (event_id, event_type, source_instance_id, timestamp, data)
		VALUES (?, ?, ?, ?, ?)`,
		e.EventId, e.EventType, e.SourceInstanceId, e.Timestamp, string(data),
	)
	return err
}

func (s *Store) GetEventHistory(eventType string, limit, offset int32) ([]*nydusv1.Event, int32, error) {
	q := `SELECT event_id, event_type, source_instance_id, timestamp, data FROM events`
	args := []any{}
	cq := `SELECT COUNT(*) FROM events`
	if eventType != "" {
		q += ` WHERE event_type = ?`
		cq += ` WHERE event_type = ?`
		args = append(args, eventType)
	}
	var total int32
	if err := s.db.QueryRow(cq, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	q += ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []*nydusv1.Event
	for rows.Next() {
		var e nydusv1.Event
		var dataJSON string
		if err := rows.Scan(&e.EventId, &e.EventType, &e.SourceInstanceId, &e.Timestamp, &dataJSON); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal([]byte(dataJSON), &e.Data)
		events = append(events, &e)
	}
	return events, total, rows.Err()
}

// ── Subscribers ──────────────────────────────────────────────────────────────

type Subscriber struct {
	InstanceID   string
	IP           string
	EventTypes   []string
	SubscribedAt int64
}

func (s *Store) UpsertSubscriber(sub *Subscriber) error {
	et, _ := json.Marshal(sub.EventTypes)
	_, err := s.db.Exec(`
		INSERT INTO subscribers (instance_id, ip, event_types, subscribed_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(instance_id) DO UPDATE SET event_types=excluded.event_types`,
		sub.InstanceID, sub.IP, string(et), sub.SubscribedAt,
	)
	return err
}

func (s *Store) DeleteSubscriber(instanceID string) error {
	_, err := s.db.Exec(`DELETE FROM subscribers WHERE instance_id = ?`, instanceID)
	return err
}

func (s *Store) ListSubscribers() ([]*nydusv1.SubscriberInfo, error) {
	rows, err := s.db.Query(`SELECT instance_id, ip, event_types, subscribed_at FROM subscribers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*nydusv1.SubscriberInfo
	for rows.Next() {
		var si nydusv1.SubscriberInfo
		var etJSON string
		if err := rows.Scan(&si.InstanceId, &si.Ip, &etJSON, &si.SubscribedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(etJSON), &si.EventTypes)
		subs = append(subs, &si)
	}
	return subs, rows.Err()
}

func nowUnix() int64 { return time.Now().Unix() }

// ── Helpers ──────────────────────────────────────────────────────────────────

func scanNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
