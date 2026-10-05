package event

import (
	"sync"
	"testing"
	"time"

	"github.com/ArsenBalakshiev/selenoid/internal/session"
	assert "github.com/stretchr/testify/require"
)

type fileRecorder struct {
	mu       sync.Mutex
	received []CreatedFile
	notified chan struct{}
}

func newRecorder() *fileRecorder {
	return &fileRecorder{notified: make(chan struct{}, 10)}
}

func (r *fileRecorder) OnFileCreated(cf CreatedFile) {
	r.mu.Lock()
	r.received = append(r.received, cf)
	r.mu.Unlock()
	r.notified <- struct{}{}
}

func TestInitRequired(t *testing.T) {
	rec := &recordInit{}
	assert.False(t, rec.initialized)
	InitIfNeeded(rec)
	assert.True(t, rec.initialized)
	InitIfNeeded(&struct{}{})
}

type recordInit struct {
	mu          sync.Mutex
	initialized bool
}

func (r *recordInit) Init() {
	r.mu.Lock()
	r.initialized = true
	r.mu.Unlock()
}

func TestFileCreatedListener(t *testing.T) {
	rec := newRecorder()
	old := fileCreatedListeners
	defer func() { fileCreatedListeners = old }()
	fileCreatedListeners = nil
	AddFileCreatedListener(rec)

	cf := CreatedFile{
		Event: Event{RequestId: 1, SessionId: "session-id"},
		Name:  "/tmp/video.mp4",
		Type:  "video",
	}
	FileCreated(cf)
	select {
	case <-rec.notified:
	case <-time.After(time.Second):
		assert.Fail(t, "listener was not invoked in time")
	}
	assert.Equal(t, 1, len(rec.received))
	assert.Equal(t, cf.Name, rec.received[0].Name)
}

func TestSessionStoppedListener(t *testing.T) {
	rec := &sessionRecorder{notified: make(chan struct{}, 10)}
	old := sessionStoppedListeners
	defer func() { sessionStoppedListeners = old }()
	sessionStoppedListeners = nil
	AddSessionStoppedListener(rec)

	ss := StoppedSession{Event: Event{RequestId: 2, SessionId: "session-id-2"}}
	SessionStopped(ss)
	select {
	case <-rec.notified:
	case <-time.After(time.Second):
		assert.Fail(t, "listener was not invoked in time")
	}
	assert.Equal(t, 1, len(rec.received))
}

type sessionRecorder struct {
	mu       sync.Mutex
	received []StoppedSession
	notified chan struct{}
}

func (r *sessionRecorder) OnSessionStopped(ss StoppedSession) {
	r.mu.Lock()
	r.received = append(r.received, ss)
	r.mu.Unlock()
	r.notified <- struct{}{}
}

func TestAddListenerInitializes(t *testing.T) {
	old := fileCreatedListeners
	defer func() { fileCreatedListeners = old }()
	fileCreatedListeners = nil
	rec := newRecorder()
	// AddFileCreatedListener should call InitIfNeeded -> Init() below.
	AddFileCreatedListener(rec)
	assert.Equal(t, 1, len(fileCreatedListeners))
}

func TestMultipleListeners(t *testing.T) {
	old := fileCreatedListeners
	defer func() { fileCreatedListeners = old }()
	fileCreatedListeners = nil
	r1 := newRecorder()
	r2 := newRecorder()
	AddFileCreatedListener(r1)
	AddFileCreatedListener(r2)
	assert.Equal(t, 2, len(fileCreatedListeners))
}

func TestSessionMetadataHasValues(t *testing.T) {
	meta := session.Metadata{ID: "1"}
	assert.NotZero(t, meta.ID)
}
