//go:build metadata
// +build metadata

package selenoid

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ArsenBalakshiev/selenoid/internal/event"
	log "github.com/ArsenBalakshiev/selenoid/internal/log"
	"github.com/ArsenBalakshiev/selenoid/internal/session"
)

const metadataFileExtension = ".json"

func init() {
	mp := &MetadataProcessor{}
	event.AddSessionStoppedListener(mp)
	log.PrintfNoId("INIT", "[Will save sessions metadata]")
}

type MetadataProcessor struct {
}

func (mp *MetadataProcessor) OnSessionStopped(stoppedSession event.StoppedSession) {
	if logOutputDir != "" {
		meta := session.Metadata{
			ID:           stoppedSession.SessionId,
			Started:      stoppedSession.Session.Started,
			Finished:     time.Now(),
			Capabilities: stoppedSession.Session.Caps,
		}
	data, err := json.MarshalIndent(meta, "", "    ")
	if err != nil {
		log.Printf(stoppedSession.RequestId, "METADATA", "[Failed to marshal: %v]", err)
		return
	}
	filename := filepath.Join(logOutputDir, stoppedSession.SessionId+metadataFileExtension)
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Printf(stoppedSession.RequestId, "METADATA", "[Failed to save to %s: %v]", filename, err)
		return
	}
	log.Printf(stoppedSession.RequestId, "METADATA", "[%s]", filename)
		createdFile := event.CreatedFile{
			Event: stoppedSession.Event,
			Name:  filename,
			Type:  "metadata",
		}
		event.FileCreated(createdFile)
	}
}
