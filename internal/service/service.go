package service

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ArsenBalakshiev/selenoid/internal/config"
	log "github.com/ArsenBalakshiev/selenoid/internal/log"
	"github.com/ArsenBalakshiev/selenoid/internal/session"
	"github.com/docker/docker/client"
)

// Environment - all settings that influence browser startup
type Environment struct {
	IP                   string
	InDocker             bool
	CPU                  int64
	Memory               int64
	Network              string
	Hostname             string
	StartupTimeout       time.Duration
	SessionDeleteTimeout time.Duration
	CaptureDriverLogs    bool
	VideoOutputDir       string
	VideoContainerImage  string
	LogOutputDir         string
	SaveAllLogs          bool
	Privileged           bool
}

const (
	DefaultContainerNetwork = "default"
)

// ServiceBase - stores fields required by all services
type ServiceBase struct {
	RequestId uint64
	Service   *config.Browser
}

// StartedService - all started service properties
type StartedService struct {
	Url       *url.URL
	Container *session.Container
	HostPort  session.HostPort
	Origin    string
	Cancel    func()
}

// Starter - interface to create session with cancellation ability
type Starter interface {
	StartWithCancel() (*StartedService, error)
}

// Manager - interface to choose appropriate starter
type Manager interface {
	Find(caps session.Caps, requestId uint64) (Starter, bool)
}

// DefaultManager - struct for default implementation
type DefaultManager struct {
	Environment *Environment
	Client      *client.Client
	Config      *config.Config
}

// Find - default implementation Manager interface
func (m *DefaultManager) Find(caps session.Caps, requestId uint64) (Starter, bool) {
	browserName := caps.BrowserName()
	version := caps.Version
	log.Printf(requestId, "LOCATING_SERVICE", "[%s] [%s]", browserName, version)
	service, version, ok := m.Config.Find(browserName, version)
	serviceBase := ServiceBase{RequestId: requestId, Service: service}
	if !ok {
		return nil, false
	}
	switch service.Image.(type) {
	case string:
		if m.Client == nil {
			return nil, false
		}
		log.Printf(requestId, "USING_DOCKER", "[%s] [%s]", browserName, version)
		return &Docker{
			ServiceBase: serviceBase,
			Environment: *m.Environment,
			Caps:        caps,
			Client:      m.Client,
			LogConfig:   m.Config.ContainerLogs}, true
	case []interface{}:
		log.Printf(requestId, "USING_DRIVER", "[%s] [%s]", browserName, version)
		return &Driver{ServiceBase: serviceBase, Environment: *m.Environment, Caps: caps}, true
	}
	return nil, false
}

func wait(u string, t time.Duration) error {
	requestTimeout := t
	if requestTimeout > time.Second {
		requestTimeout = time.Second
	}
	client := &http.Client{Timeout: requestTimeout}
	up := make(chan struct{})
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			req, _ := http.NewRequest(http.MethodHead, u, nil)
			req.Close = true
			resp, err := client.Do(req)
			if resp != nil {
				_ = resp.Body.Close()
			}
			if err != nil || resp.StatusCode >= http.StatusInternalServerError {
				<-time.After(50 * time.Millisecond)
				continue
			}
			up <- struct{}{}
			return
		}
	}()
	select {
	case <-time.After(t):
		close(done)
		return fmt.Errorf("%s does not respond in %v", u, t)
	case <-up:
	}
	return nil
}
