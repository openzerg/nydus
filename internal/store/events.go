package store

import (
	"context"
	"encoding/json"
	"time"

	nydusv1 "github.com/openzerg/common/gen/nydus/v1"
	"github.com/uptrace/bun"
)

// ── Models ─────────────────────────────────────────────────────────────────────

type Event struct {
	bun.BaseModel `bun:"table:events,alias:e"`

	EventID          string `bun:"event_id,pk"`
	EventType        string `bun:"event_type,notnull"`
	SourceInstanceID string `bun:"source_instance_id,default:''"`
	Timestamp        int64  `bun:"timestamp,notnull"`
	Data             string `bun:"data,type:text,default:'{}'"`
}

type Subscriber struct {
	bun.BaseModel `bun:"table:subscribers,alias:sub"`

	InstanceID   string `bun:"instance_id,pk"`
	IP           string `bun:"ip,default:''"`
	EventTypes   string `bun:"event_types,type:text,default:'[]'"`
	SubscribedAt int64  `bun:"subscribed_at,notnull"`
}

// ── Events ─────────────────────────────────────────────────────────────────────

func (s *Store) SaveEvent(e *nydusv1.Event) error {
	data, _ := json.Marshal(e.Data)
	row := &Event{
		EventID:          e.EventId,
		EventType:        e.EventType,
		SourceInstanceID: e.SourceInstanceId,
		Timestamp:        e.Timestamp,
		Data:             string(data),
	}
	_, err := s.db.NewInsert().Model(row).Exec(context.Background())
	return err
}

func (s *Store) GetEventHistory(eventType string, limit, offset int32) ([]*nydusv1.Event, int32, error) {
	ctx := context.Background()
	if limit <= 0 {
		limit = 50
	}

	type eventRow struct {
		EventID          string `bun:"event_id"`
		EventType        string `bun:"event_type"`
		SourceInstanceID string `bun:"source_instance_id"`
		Timestamp        int64  `bun:"timestamp"`
		Data             string `bun:"data"`
	}

	q := s.db.NewSelect().TableExpr("events").
		ColumnExpr("event_id, event_type, source_instance_id, timestamp, data")
	if eventType != "" {
		q = q.Where("event_type = ?", eventType)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	var rows []eventRow
	if err := q.OrderExpr("timestamp DESC").Limit(int(limit)).Offset(int(offset)).Scan(ctx, &rows); err != nil {
		return nil, 0, err
	}

	events := make([]*nydusv1.Event, len(rows))
	for i, r := range rows {
		ev := &nydusv1.Event{
			EventId: r.EventID, EventType: r.EventType,
			SourceInstanceId: r.SourceInstanceID, Timestamp: r.Timestamp,
		}
		_ = json.Unmarshal([]byte(r.Data), &ev.Data)
		events[i] = ev
	}
	return events, int32(total), nil
}

// ── Subscribers ───────────────────────────────────────────────────────────────

func (s *Store) UpsertSubscriber(instanceID, ip string, eventTypes []string) error {
	et, _ := json.Marshal(eventTypes)
	row := &Subscriber{
		InstanceID:   instanceID,
		IP:           ip,
		EventTypes:   string(et),
		SubscribedAt: time.Now().Unix(),
	}
	_, err := s.db.NewInsert().Model(row).
		On("CONFLICT (instance_id) DO UPDATE SET event_types = EXCLUDED.event_types").
		Exec(context.Background())
	return err
}

func (s *Store) DeleteSubscriber(instanceID string) error {
	_, err := s.db.NewDelete().TableExpr("subscribers").
		Where("instance_id = ?", instanceID).
		Exec(context.Background())
	return err
}

func (s *Store) ListSubscribers() ([]*nydusv1.SubscriberInfo, error) {
	ctx := context.Background()
	type subRow struct {
		InstanceID   string `bun:"instance_id"`
		IP           string `bun:"ip"`
		EventTypes   string `bun:"event_types"`
		SubscribedAt int64  `bun:"subscribed_at"`
	}
	var rows []subRow
	if err := s.db.NewSelect().TableExpr("subscribers").
		ColumnExpr("instance_id, ip, event_types, subscribed_at").
		Scan(ctx, &rows); err != nil {
		return nil, err
	}
	subs := make([]*nydusv1.SubscriberInfo, len(rows))
	for i, r := range rows {
		si := &nydusv1.SubscriberInfo{
			InstanceId:   r.InstanceID,
			Ip:           r.IP,
			SubscribedAt: r.SubscribedAt,
		}
		_ = json.Unmarshal([]byte(r.EventTypes), &si.EventTypes)
		subs[i] = si
	}
	return subs, nil
}
