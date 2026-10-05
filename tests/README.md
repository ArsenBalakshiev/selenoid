# Selenoid end-to-end tests (Python + pytest)

Two groups:

* `tests/test_api.py` — hub HTTP API: status, ping, session lifecycle, proxy,
  404/400 handling, queue limit with `X-Selenoid-No-Wait`. No Selenium involved.
* `tests/test_browser.py` — real browser sessions through the hub: navigation,
  JS, screenshots, cookies, windows. Every test is repeated for each browser
  in `SELENOID_BROWSERS` (default: `chrome`).
* `tests/test_video.py` — session video recording + `/video` API.
* `tests/test_logs.py` — per-session logs + `/logs` API.
* `tests/test_smoke.py` — original smoke tests (kept for compatibility).
* `tests/test_stability.py` — concurrent sessions / queue under load.

## Start Selenoid

The Dockerfile expects a prebuilt linux binary (the project builds binaries outside
of Docker):

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/selenoid_linux_amd64 ./cmd/selenoid
docker compose up -d --build
curl -s http://localhost:4444/status
```

Browser images used are declared in the root `browsers.json`; the recorded
video/logs live in the host directory `/opt/selenoid/video` and
`/opt/selenoid/logs` (the same absolute path is used by the sibling
video-recorder containers via `OVERRIDE_VIDEO_OUTPUT_DIR`, because Docker
resolves sibling bind sources on the host).

## Run tests

```bash
python -m venv .venv
source .venv/bin/activate         # Windows: .venv\Scripts\activate
pip install -r tests/requirements.txt
pytest tests/ -v
```

## Environment variables

| Variable           | Default                       | Description                          |
|--------------------|-------------------------------|--------------------------------------|
| `SELENOID_URL`     | `http://localhost:4444/wd/hub`| Selenoid hub URL                     |
| `SELENOID_BROWSERS`| `chrome`                      | Comma-separated browsers, e.g. `chrome,firefox`: each Selenium test runs once per browser |
| `SELENOID_BROWSER` | `chrome`                      | Single browser (used by API tests)   |
| `SELENOID_BROWSER_VERSION` | _not sent_            | Browser version (geckodriver rejects aliases like `latest`, use e.g. `124.0`) |
| `SELENOID_SCREEN`  | `1920x1080x24`                | Screen resolution (WxHxD)            |
| `SELENOID_VIDEO_BROWSER` | `firefox`               | Browser for the video test |
| `SELENOID_VIDEO_BROWSER_VERSION` | _not sent_      | Browser version for the video test, e.g. `latest-vnc` |

Video targets verified by e2e (all record real MP4 files):

* `selenoid/vnc:firefox_124.0` (default, no version) — official image.
* `chrome-latest-vnc` and `yandex-latest-vnc`
  (`SELENOID_VIDEO_BROWSER=chrome` + `SELENOID_VIDEO_BROWSER_VERSION=latest-vnc`
  or `yandex-vnc`) — custom images.

The recorder connects over X11 TCP (`xset -display <host>:99` → port 6099),
so the browser image must run `Xvfb ... -listen tcp`. The current image builds
also serialize the image startup: `x11vnc`/`openbox` start strictly after the
X display is ready (`xdpyinfo` wait loop, capped at 5s), which eliminates the
earlier race where `x11vnc` died at startup. Independently verified here:
12/12 starts per image with both ports (5900, 6099) live.

Notes:
* `SELENOID_BROWSERS` drives the Selenium tests only; `test_api.py` and
  `test_video.py` use `SELENOID_VIDEO_BROWSER` and the plain browser names
  (`SELENOID_BROWSER`) directly.
* `test_api.py::test_queue_full_no_wait_rejects_immediately` assumes the
  hub runs with the default `-limit 5` from compose.
* The video test keeps its session alive for ~45s: ffmpeg needs several
  seconds to produce a real MP4 payload.

Example against a remote selenoid with firefox:

```bash
SELENOID_URL=http://selenoid.example.com:4444/wd/hub SELENOID_BROWSERS=firefox pytest tests/ -v
```

Screenshots of failed tests are saved to `tests/artifacts/`.
