package handler

import (
	"sync"

	nydusv1 "github.com/openzerg/common/gen/nydus/v1"
)

// eventFanout manages in-memory event subscriptions and fan-out delivery.
type eventFanout struct {
	mu   sync.RWMutex
	subs map[string]*eventSub // keyed by instanceID
}

type eventSub struct {
	instanceID string
	eventTypes map[string]bool // empty = subscribe to all
	ch         chan *nydusv1.Event
}

func newEventFanout() *eventFanout {
	return &eventFanout{subs: make(map[string]*eventSub)}
}

func (f *eventFanout) subscribe(instanceID string, eventTypes []string) <-chan *nydusv1.Event {
	ch := make(chan *nydusv1.Event, 64)
	types := make(map[string]bool, len(eventTypes))
	for _, t := range eventTypes {
		types[t] = true
	}
	f.mu.Lock()
	f.subs[instanceID] = &eventSub{instanceID: instanceID, eventTypes: types, ch: ch}
	f.mu.Unlock()
	return ch
}

func (f *eventFanout) unsubscribe(instanceID string) {
	f.mu.Lock()
	if sub, ok := f.subs[instanceID]; ok {
		delete(f.subs, instanceID)
		close(sub.ch)
	}
	f.mu.Unlock()
}

func (f *eventFanout) publish(event *nydusv1.Event) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, sub := range f.subs {
		if len(sub.eventTypes) == 0 || sub.eventTypes[event.EventType] {
			select {
			case sub.ch <- event:
			default: // slow subscriber — drop
			}
		}
	}
}

// messageFanout manages per-chatroom message subscriptions.
type messageFanout struct {
	mu         sync.RWMutex
	subs       map[string]map[string]chan *nydusv1.Message // chatroomID → instanceID → chan
	typingSubs map[string]map[string]chan *nydusv1.TypingEvent
}

func newMessageFanout() *messageFanout {
	return &messageFanout{subs: make(map[string]map[string]chan *nydusv1.Message)}
}

func (f *messageFanout) subscribe(chatroomID, instanceID string) <-chan *nydusv1.Message {
	ch := make(chan *nydusv1.Message, 64)
	f.mu.Lock()
	if f.subs[chatroomID] == nil {
		f.subs[chatroomID] = make(map[string]chan *nydusv1.Message)
	}
	f.subs[chatroomID][instanceID] = ch
	f.mu.Unlock()
	return ch
}

func (f *messageFanout) unsubscribe(chatroomID, instanceID string) {
	f.mu.Lock()
	if room, ok := f.subs[chatroomID]; ok {
		if ch, ok := room[instanceID]; ok {
			delete(room, instanceID)
			close(ch)
		}
	}
	f.mu.Unlock()
}

func (f *messageFanout) publish(msg *nydusv1.Message) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, ch := range f.subs[msg.ChatroomId] {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (f *messageFanout) subscribeTyping(chatroomID string) <-chan *nydusv1.TypingEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.subs[chatroomID] == nil {
		f.subs[chatroomID] = make(map[string]chan *nydusv1.Message)
	}
	ch := make(chan *nydusv1.TypingEvent, 16)
	if f.typingSubs == nil {
		f.typingSubs = make(map[string]map[string]chan *nydusv1.TypingEvent)
	}
	if f.typingSubs[chatroomID] == nil {
		f.typingSubs[chatroomID] = make(map[string]chan *nydusv1.TypingEvent)
	}
	instanceID := make([]byte, 4)
	f.typingSubs[chatroomID][string(instanceID)] = ch
	return ch
}

func (f *messageFanout) unsubscribeTyping(chatroomID string) {
	f.mu.Lock()
	if f.typingSubs != nil && f.typingSubs[chatroomID] != nil {
		for _, ch := range f.typingSubs[chatroomID] {
			close(ch)
		}
		delete(f.typingSubs, chatroomID)
	}
	f.mu.Unlock()
}

func (f *messageFanout) publishTyping(chatroomID string, event *nydusv1.TypingEvent) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.typingSubs == nil || f.typingSubs[chatroomID] == nil {
		return
	}
	for _, ch := range f.typingSubs[chatroomID] {
		select {
		case ch <- event:
		default:
		}
	}
}
