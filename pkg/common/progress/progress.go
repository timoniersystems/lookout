package progress

import (
	"sync"
	"time"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusActive   Status = "active"
	StatusComplete Status = "complete"
	StatusError    Status = "error"
)

type Update struct {
	Type     string `json:"type"`     // "progress", "complete", "error"
	Step     string `json:"step"`     // upload, parse, db, scan, cve, paths, complete
	Status   Status `json:"status"`   // pending, active, complete, error
	Message  string `json:"message"`  // Human-readable message
	Progress int    `json:"progress"` // 0-100
	Redirect string `json:"redirect,omitempty"`
}

type Tracker struct {
	SessionID string
	Updates   chan Update
	mu        sync.Mutex
	closed    bool
}

var (
	trackers = make(map[string]*Tracker)
	mu       sync.RWMutex
)

func NewTracker(sessionID string) *Tracker {
	tracker := &Tracker{
		SessionID: sessionID,
		Updates:   make(chan Update, 100),
	}

	mu.Lock()
	trackers[sessionID] = tracker
	mu.Unlock()

	// Auto-cleanup after 10 minutes
	go func() {
		time.Sleep(10 * time.Minute)
		tracker.Close()
		mu.Lock()
		delete(trackers, sessionID)
		mu.Unlock()
	}()

	return tracker
}

func GetTracker(sessionID string) *Tracker {
	mu.RLock()
	defer mu.RUnlock()
	return trackers[sessionID]
}

func (t *Tracker) SendProgress(step string, status Status, message string, progress int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}

	select {
	case t.Updates <- Update{
		Type:     "progress",
		Step:     step,
		Status:   status,
		Message:  message,
		Progress: progress,
	}:
	default:
		// Channel full, skip update
	}
}

func (t *Tracker) SendComplete(redirect string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}

	select {
	case t.Updates <- Update{
		Type:     "complete",
		Redirect: redirect,
	}:
	default:
	}

	t.Close()
}

func (t *Tracker) SendError(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}

	select {
	case t.Updates <- Update{
		Type:    "error",
		Message: message,
	}:
	default:
	}

	t.Close()
}

func (t *Tracker) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.closed {
		t.closed = true
		close(t.Updates)
	}
}
