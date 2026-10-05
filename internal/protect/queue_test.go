package protect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

func TestNewQueueCounters(t *testing.T) {
	q := New(2, false)
	assert.Equal(t, 0, q.Used())
	assert.Equal(t, 0, q.Pending())
	assert.Equal(t, 0, q.Queued())
}

// acquireSlot fills one limit slot and puts a request into pending,
// mimicking the correct Protect+Create usage. Returns true if the slot
// channel was already full.
func (q *Queue) acquireSlot() bool {
	select {
	case q.limit <- struct{}{}:
		q.pending <- struct{}{}
		return false
	default:
		return true
	}
}

func TestQueueCreateAndRelease(t *testing.T) {
	q := New(1, false)
	assert.False(t, q.acquireSlot())
	q.Create()
	assert.Equal(t, 1, q.Used())
	assert.Equal(t, 0, q.Pending())

	// The slot is still occupied until Release().
	assert.True(t, q.acquireSlot())

	q.Release()
	assert.Equal(t, 0, q.Used())
	assert.False(t, q.acquireSlot())
}

func TestQueueDrop(t *testing.T) {
	q := New(1, false)
	assert.False(t, q.acquireSlot())
	q.Drop()
	assert.Equal(t, 0, q.Pending())
	// Slot is free again.
	assert.False(t, q.acquireSlot())
}

func TestTryPassesThroughWhenNotFull(t *testing.T) {
	q := New(1, false)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handlerCalled := false
	q.Try(func(http.ResponseWriter, *http.Request) { handlerCalled = true })(w, r)
	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTryFullQueueNoWaitHeader(t *testing.T) {
	q := New(1, true)
	q.acquireSlot()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Selenoid-No-Wait", "true")
	handlerCalled := false
	q.Try(func(http.ResponseWriter, *http.Request) { handlerCalled = true })(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.False(t, handlerCalled)
}

func TestTryFullQueueNoHeaderJustWaiting(t *testing.T) {
	q := New(1, false)
	q.acquireSlot()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handlerCalled := false
	q.Try(func(http.ResponseWriter, *http.Request) { handlerCalled = true })(w, r)
	assert.True(t, handlerCalled)
}

func TestCheckDisabledQueueFull(t *testing.T) {
	q := New(1, true) // disabled
	q.acquireSlot()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handlerCalled := false
	q.Check(func(http.ResponseWriter, *http.Request) { handlerCalled = true })(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.False(t, handlerCalled)
}

func TestCheckEnabledQueueDoesNotBlock(t *testing.T) {
	q := New(1, true)
	// Free slot: handler should be called.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handlerCalled := false
	q.Check(func(http.ResponseWriter, *http.Request) { handlerCalled = true })(w, r)
	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProtectOneRequest(t *testing.T) {
	q := New(1, false)
	inside := make(chan struct{})
	release := make(chan struct{})
	handler := func(_ http.ResponseWriter, _ *http.Request) {
		close(inside)
		<-release
	}
	srv := httptest.NewServer(q.Protect(handler))
	defer srv.Close()

	served := make(chan struct{})
	go func() {
		http.Get(srv.URL)
		served <- struct{}{}
	}()
	<-inside
	assert.Equal(t, 0, q.Used())
	assert.Equal(t, 1, q.Pending())
	close(release)
	q.Drop()
	<-served
}

func TestProtectLimit(t *testing.T) {
	q := New(1, false)
	block := make(chan struct{})
	handlerDone := make(chan struct{}, 20)
	handler := func(_ http.ResponseWriter, _ *http.Request) {
		<-block
		q.Create()
		handlerDone <- struct{}{}
	}
	srv := httptest.NewServer(q.Protect(handler))
	defer srv.Close()

	// One request in flight: fills the slot.
	go func() { _, _ = http.Get(srv.URL) }()
	assert.Eventually(t, func() bool { return q.Pending() == 1 }, time.Second, 5*time.Millisecond)

	// The single slot is taken; a real second request must wait in queue.
	second := make(chan struct{})
	go func() {
		_, _ = http.Get(srv.URL)
		second <- struct{}{}
	}()
	assert.Eventually(t, func() bool { return q.Queued() >= 1 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, 1, len(q.limit))

	// First request completes, releases the slot -> queued request proceeds.
	close(block)
	<-handlerDone
	q.Release()
	<-second
	q.Release()
	assert.Equal(t, 0, q.Used())
}

func TestProtectClientDisconnected(t *testing.T) {
	q := New(1, false)
	handler := func(_ http.ResponseWriter, _ *http.Request) {}
	srv := httptest.NewServer(q.Protect(handler))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	done := make(chan struct{})
	go func() {
		_, _ = http.DefaultClient.Do(req)
		close(done)
	}()
	// Poll until the request acquired the limit slot.
	assert.Eventually(t, func() bool { return len(q.limit) == 1 || q.Pending() > 0 }, time.Second, 5*time.Millisecond)
	cancel()
	<-done
	assert.Equal(t, 0, q.Used())
}

func TestQueueConcurrentProtect(t *testing.T) {
	const limit = 5
	q := New(limit, false)
	var mu sync.Mutex
	var inside, maxInside int
	handler := func(_ http.ResponseWriter, _ *http.Request) {
		q.Create()
		mu.Lock()
		inside++
		if inside > maxInside {
			maxInside = inside
		}
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		inside--
		mu.Unlock()
		q.Release()
	}
	srv := httptest.NewServer(q.Protect(handler))
	defer srv.Close()

	const total = 20
	wg := &sync.WaitGroup{}
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rsp, err := http.Get(srv.URL)
			if err == nil {
				_ = rsp.Body.Close()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 0, q.Used())
	assert.Equal(t, 0, q.Pending())
	assert.LessOrEqual(t, maxInside, limit)
}
