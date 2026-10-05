package upload

import (
	"sync"
	"testing"
	"time"

	"github.com/ArsenBalakshiev/selenoid/internal/event"
	assert "github.com/stretchr/testify/require"
)

type mockUploader struct {
	mu       sync.Mutex
	uploaded bool
	err      error
	done     chan struct{}
}

func (m *mockUploader) Upload(cf event.CreatedFile) (bool, error) {
	m.mu.Lock()
	m.uploaded = true
	m.mu.Unlock()
	if m.done != nil {
		m.done <- struct{}{}
	}
	return true, m.err
}

func createdFile() event.CreatedFile {
	return event.CreatedFile{
		Event: event.Event{RequestId: 42, SessionId: "session-id"},
		Name:  "/tmp/some-file.mp4",
		Type:  "video",
	}
}

func TestOnFileCreatedWithoutUploaders(t *testing.T) {
	(&Upload{}).OnFileCreated(createdFile())
}

func TestOnFileCreatedInvokesUploaders(t *testing.T) {
	done := make(chan struct{}, 2)
	u1 := &mockUploader{done: done}
	u2 := &mockUploader{done: done}
	ul := &Upload{uploaders: []Uploader{u1, u2}}

	ul.OnFileCreated(createdFile())
	<-done
	<-done
	assert.True(t, u1.uploaded)
	assert.True(t, u2.uploaded)
}

func TestAddUploader(t *testing.T) {
	oldUpl := upl
	defer func() { upl = oldUpl }()
	upl = nil
	defer func() { upl = oldUpl }()

	u := &mockUploader{}
	AddUploader(u)
	assert.NotNil(t, upl)
	assert.Equal(t, 1, len(upl.uploaders))
	AddUploader(&mockUploader{})
	assert.Equal(t, 2, len(upl.uploaders))
}

func TestInitNoUploaders(t *testing.T) {
	oldUpl := upl
	defer func() { upl = oldUpl }()
	upl = nil
	// Should not panic with no uploaders registered.
	Init()
}

func TestOnFileCreatedUploadError(t *testing.T) {
	done := make(chan struct{}, 1)
	u := &mockUploader{done: done}
	ul := &Upload{uploaders: []Uploader{u}}

	ul.OnFileCreated(createdFile())
	select {
	case <-done:
	case <-time.After(time.Second):
		assert.Fail(t, "uploader was not called")
	}
	assert.True(t, u.uploaded)
}

func TestUploaderNotFlaggedUploaded(t *testing.T) {
	// Uploader returning false (not uploaded) should not error and be logged as skipped.
	u := &notUploaded{}
	ul := &Upload{uploaders: []Uploader{u}}
	done := make(chan struct{})
	_ = done
	ul.OnFileCreated(createdFile())
}

type notUploaded struct{}

func (n *notUploaded) Upload(_ event.CreatedFile) (bool, error) { return false, nil }
