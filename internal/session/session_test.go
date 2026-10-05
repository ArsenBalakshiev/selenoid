package session

import (
	"encoding/json"
	"sync"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestNewMap(t *testing.T) {
	m := NewMap()
	assert.Equal(t, 0, m.Len())
}

func TestMapPutGetRemove(t *testing.T) {
	m := NewMap()
	m.Put("id1", &Session{Quota: "user1"})
	m.Put("id2", &Session{Quota: "user2"})
	assert.Equal(t, 2, m.Len())

	s, ok := m.Get("id1")
	assert.True(t, ok)
	assert.Equal(t, "user1", s.Quota)

	m.Remove("id1")
	_, ok = m.Get("id1")
	assert.False(t, ok)
	assert.Equal(t, 1, m.Len())
}

func TestMapGetMissing(t *testing.T) {
	m := NewMap()
	_, ok := m.Get("missing")
	assert.False(t, ok)
}

func TestMapEach(t *testing.T) {
	m := NewMap()
	m.Put("id1", &Session{Quota: "user1"})
	m.Put("id2", &Session{Quota: "user2"})
	seen := map[string]bool{}
	m.Each(func(k string, s *Session) {
		seen[k] = true
	})
	assert.Len(t, seen, 2)
}

func TestMapConcurrentAccess(t *testing.T) {
	m := NewMap()
	wg := &sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "id" + string(rune('a'+i%5))
			m.Put(key, &Session{Quota: "user"})
			_, _ = m.Get(key)
			m.Each(func(_ string, _ *Session) {})
			m.Remove(key)
			_ = m.Len()
		}(i)
	}
	wg.Wait()
}

func TestCapsBrowserNamePrecedence(t *testing.T) {
	caps := Caps{Name: "firefox", DeviceName: "android", W3CDeviceName: "iphone"}
	assert.Equal(t, "firefox", caps.BrowserName())

	caps = Caps{DeviceName: "android", W3CDeviceName: "iphone"}
	assert.Equal(t, "android", caps.BrowserName())

	caps = Caps{W3CDeviceName: "iphone"}
	assert.Equal(t, "iphone", caps.BrowserName())
}

func TestProcessExtensionCapabilities(t *testing.T) {
	caps := Caps{
		Version: "57.0",
		ExtensionCapabilities: &Caps{
			TestName: "test-name",
			VNC:      true,
			Env:      []string{"FOO=bar"},
		},
	}
	caps.ProcessExtensionCapabilities()
	assert.Equal(t, "test-name", caps.TestName)
	assert.True(t, caps.VNC)
	assert.Equal(t, []string{"FOO=bar"}, caps.Env)
}

func TestProcessExtensionCapabilitiesNil(t *testing.T) {
	var caps Caps
	assert.NotPanics(t, caps.ProcessExtensionCapabilities)
}

func TestProcessExtensionCapabilitiesW3CAliases(t *testing.T) {
	caps := Caps{
		W3CVersion:  "100.0",
		W3CPlatform: "linux",
	}
	caps.ProcessExtensionCapabilities()
	assert.Equal(t, "100.0", caps.Version)
	assert.Equal(t, "linux", caps.Platform)
}

func TestCapsJSONRoundtrip(t *testing.T) {
	raw := `{"browserName":"chrome","version":"100.0","enableVNC":true}`
	var caps Caps
	assert.NoError(t, json.Unmarshal([]byte(raw), &caps))
	assert.Equal(t, "chrome", caps.Name)
	assert.Equal(t, "100.0", caps.Version)
	assert.True(t, caps.VNC)
}
