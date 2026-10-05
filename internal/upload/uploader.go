package upload

import (
	"time"

	"github.com/ArsenBalakshiev/selenoid/internal/event"
	"github.com/ArsenBalakshiev/selenoid/internal/info"
	log "github.com/ArsenBalakshiev/selenoid/internal/log"
)

var (
	upl *Upload
)

type Uploader interface {
	Upload(createdFile event.CreatedFile) (bool, error)
}

type Upload struct {
	uploaders []Uploader
}

func Init() {
	if upl != nil {
		for _, u := range upl.uploaders {
			event.InitIfNeeded(u)
		}
	}
}

func AddUploader(u Uploader) {
	if upl == nil {
		upl = &Upload{}
		event.AddFileCreatedListener(upl)
	}
	upl.uploaders = append(upl.uploaders, u)
}

func (ul *Upload) OnFileCreated(createdFile event.CreatedFile) {
	if len(ul.uploaders) == 0 {
		return
	}
	for _, uploader := range ul.uploaders {
		go func(u Uploader) {
			s := time.Now()
			uploaded, err := u.Upload(createdFile)
			if err != nil {
				log.Printf(createdFile.RequestId, "UPLOADING_FILE", "[%s] [Failed to upload: %v]", createdFile.Name, err)
				return
			}
			if uploaded {
				log.Printf(createdFile.RequestId, "UPLOADED_FILE", "[%s] [%.2fs]", createdFile.Name, info.SecondsSince(s))
			}
		}(uploader)
	}
}
