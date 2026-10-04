# Selenoid smoke tests (Python + Selenium)

Basic end-to-end tests running against a Selenoid hub started via `docker-compose.yml`
in the repository root.

## Start Selenoid

The Dockerfile expects a prebuilt linux binary (the project builds binaries outside
of Docker):

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/selenoid_linux_amd64 ./cmd/selenoid
docker compose up -d --build
curl -s http://localhost:4444/status
```

Browser images used are declared in the root `browsers.json`.

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
| `SELENOID_BROWSER` | `chrome`                      | Browser from `browsers.json`         |
| `SELENOID_BROWSER_VERSION` | _not sent_            | Browser version (geckodriver rejects aliases like `latest`, use e.g. `124.0`) |
| `SELENOID_SCREEN`  | `1920x1080x24`                | Screen resolution (WxHxD)            |

Example against a remote selenoid with firefox:

```bash
SELENOID_URL=http://selenoid.example.com:4444/wd/hub SELENOID_BROWSER=firefox pytest tests/ -v
```

Screenshots of failed tests are saved to `tests/artifacts/`.
