package selenoid

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	ggr "github.com/aerokube/ggr/config"
	"github.com/ArsenBalakshiev/selenoid/internal/config"
	"github.com/ArsenBalakshiev/selenoid/internal/info"
	"github.com/ArsenBalakshiev/selenoid/internal/jsonerror"
	log "github.com/ArsenBalakshiev/selenoid/internal/log"
	"github.com/ArsenBalakshiev/selenoid/internal/protect"
	"github.com/ArsenBalakshiev/selenoid/internal/service"
	"github.com/ArsenBalakshiev/selenoid/internal/session"
	"github.com/ArsenBalakshiev/selenoid/internal/upload"
	"github.com/docker/docker/api"
	"github.com/docker/docker/client"
	"golang.org/x/net/websocket"
)

var (
	hostname                 string
	disableDocker            bool
	disableQueue             bool
	enableFileUpload         bool
	listen                   string
	timeout                  time.Duration
	maxTimeout               time.Duration
	newSessionAttemptTimeout time.Duration
	sessionDeleteTimeout     time.Duration
	serviceStartupTimeout    time.Duration
	gracefulPeriod           time.Duration
	limit                    int
	retryCount               int
	containerNetwork         string
	sessions                 = session.NewMap()
	confPath                 string
	logConfPath              string
	captureDriverLogs        bool
	disablePrivileged        bool
	videoOutputDir           string
	videoRecorderImage       string
	logOutputDir             string
	saveAllLogs              bool
	ggrHost                  *ggr.Host
	conf                     *config.Config
	queue                    *protect.Queue
	manager                  service.Manager
	cli                      *client.Client
	mem                      service.MemLimit
	cpu                      service.CpuLimit

	startTime = time.Now()

	version     bool
	gitRevision = "HEAD"
	buildStamp  = "unknown"
)

var (
	once sync.Once
)

// init registers command line flags. Parsing happens in Run(), so importing
// this package from tests does not require a working environment.
func init() {
	flags()
}

func flags() {
	if flag.Parsed() {
		return
	}
	flag.BoolVar(&disableDocker, "disable-docker", false, "Disable docker support")
	flag.BoolVar(&disableQueue, "disable-queue", false, "Disable wait queue")
	flag.BoolVar(&enableFileUpload, "enable-file-upload", false, "File upload support")
	flag.StringVar(&listen, "listen", ":4444", "Network address to accept connections")
	flag.StringVar(&confPath, "conf", "browsers.json", "Browsers configuration file")
	flag.StringVar(&logConfPath, "log-conf", "", "Container logging configuration file")
	flag.IntVar(&limit, "limit", 5, "Simultaneous container runs")
	flag.IntVar(&retryCount, "retry-count", 1, "New session attempts retry count")
	flag.DurationVar(&timeout, "timeout", 60*time.Second, "Session idle timeout in time.Duration format")
	flag.DurationVar(&maxTimeout, "max-timeout", 1*time.Hour, "Maximum valid session idle timeout in time.Duration format")
	flag.DurationVar(&newSessionAttemptTimeout, "session-attempt-timeout", 30*time.Second, "New session attempt timeout in time.Duration format")
	flag.DurationVar(&sessionDeleteTimeout, "session-delete-timeout", 30*time.Second, "Session delete timeout in time.Duration format")
	flag.DurationVar(&serviceStartupTimeout, "service-startup-timeout", 30*time.Second, "Service startup timeout in time.Duration format")
	flag.BoolVar(&version, "version", false, "Show version and exit")
	flag.Var(&mem, "mem", "Containers memory limit e.g. 128m or 1g")
	flag.Var(&cpu, "cpu", "Containers cpu limit as float e.g. 0.2 or 1.0")
	flag.StringVar(&containerNetwork, "container-network", service.DefaultContainerNetwork, "Network to be used for containers")
	flag.BoolVar(&captureDriverLogs, "capture-driver-logs", false, "Whether to add driver process logs to Selenoid output")
	flag.BoolVar(&disablePrivileged, "disable-privileged", false, "Whether to disable privileged container mode")
	flag.StringVar(&videoOutputDir, "video-output-dir", "video", "Directory to save recorded video to")
	flag.StringVar(&videoRecorderImage, "video-recorder-image", "selenoid/video-recorder:latest-release", "Image to use as video recorder")
	flag.StringVar(&logOutputDir, "log-output-dir", "", "Directory to save session log to")
	flag.BoolVar(&saveAllLogs, "save-all-logs", false, "Whether to save all logs without considering capabilities")
	flag.DurationVar(&gracefulPeriod, "graceful-period", 300*time.Second, "graceful shutdown period in time.Duration format, e.g. 300s or 500ms")
}

// init once the flags are parsed and initializes the selenoid environment:
// it is safe to call multiple times; actual work happens only once.
func configure() {
	once.Do(configureOnce)
}

func configureOnce() {
	flag.Parse()

	if version {
		showVersion()
		os.Exit(0)
	}

	var err error
	hostname, err = os.Hostname()
	if err != nil {
		log.FatalNoId("INIT", "[%s: %v]", os.Args[0], err)
	}
	if ggrHostEnv := os.Getenv("GGR_HOST"); ggrHostEnv != "" {
		ggrHost = parseGgrHost(ggrHostEnv)
	}
	queue = protect.New(limit, disableQueue)
	conf = config.NewConfig()
	err = conf.Load(confPath, logConfPath)
	if err != nil {
		log.FatalNoId("INIT", "[%s: %v]", os.Args[0], err)
	}
	onSIGHUP(func() {
		err := conf.Load(confPath, logConfPath)
		if err != nil {
			log.PrintfNoId("INIT", "[%s: %v]", os.Args[0], err)
		}
	})
	inDocker := false
	_, err = os.Stat("/.dockerenv")
	if err == nil {
		inDocker = true
	}

	if !disableDocker {
		videoOutputDir, err = filepath.Abs(videoOutputDir)
		if err != nil {
			log.FatalNoId("INIT", "[Invalid video output dir %s: %v]", videoOutputDir, err)
		}
		err = os.MkdirAll(videoOutputDir, 0755)
		if err != nil {
			log.FatalNoId("INIT", "[Failed to create video output dir %s: %v]", videoOutputDir, err)
		}
		log.PrintfNoId("INIT", "[Video Dir: %s]", videoOutputDir)
	}
	if logOutputDir != "" {
		logOutputDir, err = filepath.Abs(logOutputDir)
		if err != nil {
			log.FatalNoId("INIT", "[Invalid log output dir %s: %v]", logOutputDir, err)
		}
		err = os.MkdirAll(logOutputDir, 0755)
		if err != nil {
			log.FatalNoId("INIT", "[Failed to create log output dir %s: %v]", logOutputDir, err)
		}
		log.PrintfNoId("INIT", "[Logs Dir: %s]", logOutputDir)
		if saveAllLogs {
			log.PrintfNoId("INIT", "[Saving all logs]")
		}
	}

	upload.Init()

	environment := service.Environment{
		InDocker:             inDocker,
		CPU:                  int64(cpu),
		Memory:               int64(mem),
		Network:              containerNetwork,
		StartupTimeout:       serviceStartupTimeout,
		SessionDeleteTimeout: sessionDeleteTimeout,
		CaptureDriverLogs:    captureDriverLogs,
		VideoOutputDir:       videoOutputDir,
		VideoContainerImage:  videoRecorderImage,
		LogOutputDir:         logOutputDir,
		SaveAllLogs:          saveAllLogs,
		Privileged:           !disablePrivileged,
	}
	if disableDocker {
		manager = &service.DefaultManager{Environment: &environment, Config: conf}
		if logOutputDir != "" && captureDriverLogs {
			log.FatalNoId("INIT", "[In drivers mode only one of -capture-driver-logs and -log-output-dir flags is allowed]")
		}
		return
	}
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = client.DefaultDockerHost
	}
	u, err := client.ParseHostURL(dockerHost)
	if err != nil {
		log.FatalNoId("INIT", "[%v]", err)
	}
	ip, _, _ := net.SplitHostPort(u.Host)
	environment.IP = ip
	cli, err = createCompatibleDockerClient(
		func(specifiedApiVersion string) {
			log.PrintfNoId("INIT", "[Using Docker API version: %s]", specifiedApiVersion)
		},
		func(determinedApiVersion string) {
			log.PrintfNoId("INIT", "[Your Docker API version is %s]", determinedApiVersion)
		},
		func(defaultApiVersion string) {
			log.PrintfNoId("INIT", "[Did not manage to determine your Docker API version - using default version: %s]", defaultApiVersion)
		},
	)
	if err != nil {
		log.FatalNoId("INIT", "[New docker client: %v]", err)
	}
	manager = &service.DefaultManager{Environment: &environment, Client: cli, Config: conf}
}

func createCompatibleDockerClient(onVersionSpecified, onVersionDetermined, onUsingDefaultVersion func(string)) (*client.Client, error) {
	const dockerApiVersion = "DOCKER_API_VERSION"
	dockerApiVersionEnv := os.Getenv(dockerApiVersion)
	if dockerApiVersionEnv != "" {
		onVersionSpecified(dockerApiVersionEnv)
	} else {
		maxMajorVersion, maxMinorVersion := parseVersion(api.DefaultVersion)
		minMajorVersion, minMinorVersion := parseVersion("1.24")
		for majorVersion := maxMajorVersion; majorVersion >= minMajorVersion; majorVersion-- {
			for minorVersion := maxMinorVersion; minorVersion >= minMinorVersion; minorVersion-- {
				apiVersion := fmt.Sprintf("%d.%d", majorVersion, minorVersion)
				_ = os.Setenv(dockerApiVersion, apiVersion)
				docker, err := client.NewClientWithOpts(client.FromEnv)
				if err != nil {
					return nil, err
				}
				if isDockerAPIVersionCorrect(docker) {
					onVersionDetermined(apiVersion)
					return docker, nil
				}
				_ = docker.Close()
			}
		}
		_ = os.Unsetenv(dockerApiVersion)
		onUsingDefaultVersion(api.DefaultVersion)
	}
	return client.NewClientWithOpts(client.FromEnv)
}

func parseVersion(ver string) (int, int) {
	const point = "."
	pieces := strings.Split(ver, point)
	major, err := strconv.Atoi(pieces[0])
	if err != nil {
		return 0, 0
	}
	minor, err := strconv.Atoi(pieces[1])
	if err != nil {
		return 0, 0
	}
	return major, minor
}

func isDockerAPIVersionCorrect(docker *client.Client) bool {
	ctx := context.Background()
	apiInfo, err := docker.ServerVersion(ctx)
	if err != nil {
		return false
	}
	serverMajor, serverMinor := parseVersion(apiInfo.APIVersion)
	clientMajor, clientMinor := parseVersion(docker.ClientVersion())
	return serverMajor > clientMajor || (serverMajor == clientMajor && serverMinor >= clientMinor)
}

func parseGgrHost(s string) *ggr.Host {
	h, p, err := net.SplitHostPort(s)
	if err != nil {
		log.FatalNoId("INIT", "[Invalid Ggr host: %v]", err)
	}
	ggrPort, err := strconv.Atoi(p)
	if err != nil {
		log.FatalNoId("INIT", "[Invalid Ggr host: %v]", err)
	}
	host := &ggr.Host{
		Name: h,
		Port: ggrPort,
	}
	log.PrintfNoId("INIT", "[Will prefix all session IDs with a hash-sum: %s]", host.Sum())
	return host
}

func onSIGHUP(fn func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP)
	go func() {
		for {
			<-sig
			fn()
		}
	}()
}

var seleniumPaths = struct {
	CreateSession, ProxySession string
}{
	CreateSession: "/session",
	ProxySession:  "/session/",
}

func selenium() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(seleniumPaths.CreateSession, post(queue.Try(queue.Check(queue.Protect(create)))))
	mux.HandleFunc(seleniumPaths.ProxySession, proxy)
	mux.HandleFunc(paths.Status, status)
	mux.HandleFunc(paths.Welcome, welcome)
	return mux
}

func post(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func ping(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Uptime         string `json:"uptime"`
		LastReloadTime string `json:"lastReloadTime"`
		NumRequests    uint64 `json:"numRequests"`
		Version        string `json:"version"`
	}{time.Since(startTime).String(), conf.LastReloadTime.Format(time.RFC3339), getSerial(), gitRevision})
}

func video(w http.ResponseWriter, r *http.Request) {
	requestId := serial()
	if r.Method == http.MethodDelete {
		deleteFileIfExists(requestId, w, r, videoOutputDir, paths.Video, "DELETED_VIDEO_FILE")
		return
	}
	user, remote := info.RequestInfo(r)
	if _, ok := r.URL.Query()[jsonParam]; ok {
		listFilesAsJson(requestId, w, videoOutputDir, "VIDEO_ERROR")
		return
	}
	log.Printf(requestId, "VIDEO_LISTING", "[%s] [%s]", user, remote)
	fileServer := http.StripPrefix(paths.Video, http.FileServer(http.Dir(videoOutputDir)))
	fileServer.ServeHTTP(w, r)
}

func deleteFileIfExists(requestId uint64, w http.ResponseWriter, r *http.Request, dir string, prefix string, status string) {
	user, remote := info.RequestInfo(r)
	fileName := strings.TrimPrefix(r.URL.Path, prefix)
	filePath := filepath.Join(dir, fileName)
	_, err := os.Stat(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unknown file %s", filePath), http.StatusNotFound)
		return
	}
	err = os.Remove(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete file %s: %v", filePath, err), http.StatusInternalServerError)
		return
	}
	log.Printf(requestId, status, "[%s] [%s] [%s]", user, remote, fileName)
}

var paths = struct {
	Video, VNC, Logs, Devtools, Download, Clipboard, File, Ping, Status, Error, WdHub, Welcome string
}{
	Video:     "/video/",
	VNC:       "/vnc/",
	Logs:      "/logs/",
	Devtools:  "/devtools/",
	Download:  "/download/",
	Clipboard: "/clipboard/",
	Status:    "/status",
	File:      "/file",
	Ping:      "/ping",
	Error:     "/error",
	WdHub:     "/wd/hub",
	Welcome:   "/",
}

func handler() http.Handler {
	root := http.NewServeMux()
	root.HandleFunc(paths.WdHub+"/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		r.URL.Scheme = "http"
		r.URL.Host = (&request{r}).localaddr()
		r.URL.Path = strings.TrimPrefix(r.URL.Path, paths.WdHub)
		selenium().ServeHTTP(w, r)
	})
	root.HandleFunc(paths.Error, func(w http.ResponseWriter, r *http.Request) {
		jsonerror.InvalidSessionID(errors.New("session timed out or not found")).Encode(w)
	})
	root.HandleFunc(paths.Status, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(conf.State(sessions, limit, queue.Queued(), queue.Pending()))
	})
	root.HandleFunc(paths.Ping, ping)
	root.Handle(paths.VNC, websocket.Handler(vnc))
	root.HandleFunc(paths.Logs, logs)
	root.HandleFunc(paths.Video, video)
	root.HandleFunc(paths.Download, reverseProxy(func(sess *session.Session) string { return sess.HostPort.Fileserver }, "DOWNLOADING_FILE"))
	root.HandleFunc(paths.Clipboard, reverseProxy(func(sess *session.Session) string { return sess.HostPort.Clipboard }, "CLIPBOARD"))
	root.HandleFunc(paths.Devtools, reverseProxy(func(sess *session.Session) string { return sess.HostPort.Devtools }, "DEVTOOLS"))
	if enableFileUpload {
		root.HandleFunc(paths.File, fileUpload)
	}
	root.HandleFunc(paths.Welcome, welcome)
	return root
}

func showVersion() {
	fmt.Printf("Git Revision: %s\n", gitRevision)
	fmt.Printf("UTC Build Time: %s\n", buildStamp)
}

func Run() {
	configure()
	log.PrintfNoId("INIT", "[Timezone: %s]", time.Local)
	log.PrintfNoId("INIT", "[Listening on %s]", listen)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	server := &http.Server{
		Addr:    listen,
		Handler: handler(),
	}
	e := make(chan error)
	go func() {
		e <- server.ListenAndServe()
	}()
	select {
	case err := <-e:
		log.FatalNoId("INIT", "[Failed to start: %v]", err)
	case <-stop:
	}

	log.PrintfNoId("SHUTTING_DOWN", "[%s]", gracefulPeriod)
	ctx, cancel := context.WithTimeout(context.Background(), gracefulPeriod)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.FatalNoId("SHUTTING_DOWN", "[Failed to shut down: %v]", err)
	}

	sessions.Each(func(k string, s *session.Session) {
		if enableFileUpload {
			_ = os.RemoveAll(path.Join(os.TempDir(), k))
		}
		s.Cancel()
	})

	if !disableDocker {
		err := cli.Close()
		if err != nil {
			log.FatalNoId("SHUTTING_DOWN", "[Error closing Docker client: %v]", err)
		}
	}
}
