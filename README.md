# Selenoid
[![Build Status](https://github.com/ArsenBalakshiev/selenoid/actions/workflows/ci.yml/badge.svg)](https://github.com/ArsenBalakshiev/selenoid/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/release/ArsenBalakshiev/selenoid.svg)](https://github.com/ArsenBalakshiev/selenoid/releases/latest)

Selenoid is a powerful implementation of [Selenium](http://github.com/SeleniumHQ/selenium) hub using [Docker](https://docker.com/) containers to launch browsers.
![Selenoid Animation](docs/img/selenoid-animation.gif)

## Features

### Runs in a Container
Start browser automation in minutes by running a single container:
```
$ docker run -d --name selenoid -p 4444:4444 \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v $(pwd)/browsers.json:/etc/selenoid/browsers.json:ro \
    -v $(pwd)/video:/opt/selenoid/video \
    ghcr.io/arsenbalakshiev/selenoid:latest
```
**That's it!** You can now use Selenoid instead of Selenium server. Specify the following Selenium URL in tests:
```
http://localhost:4444/wd/hub
```

### Ready to use Browser Images
No need to manually install browsers or dive into WebDriver documentation. Available images (see [browsers.json](browsers.json)):

| Browser        | Image                                                   |
|----------------|----------------------------------------------------------|
| Chrome         | `ghcr.io/arsenbalakshiev/selenoid-images:chrome-latest` |
| Yandex Browser | `ghcr.io/arsenbalakshiev/selenoid-images:yandex-latest` |

Image tags are floating and updated with each rebuild. Yandex Browser is requested in tests with `browserName: chrome` and `version: yandex`.

### Live Browser Screen and Logs
New **[rich user interface](https://github.com/aerokube/selenoid-ui)** showing browser screen and Selenium session logs:
![Selenoid UI](docs/img/selenoid-ui.png)

### Video Recording
* Any browser session can be saved to [H.264](https://en.wikipedia.org/wiki/H.264/MPEG-4_AVC) video ([example](https://www.youtube.com/watch?v=maB298oO5cI))
* An API to list, download and delete recorded video files

### Convenient Logging

* Any browser session logs are automatically saved to files - one per session
* An API to list, download and delete saved log files

### Lightweight and Lightning Fast
Suitable for personal usage and in big clusters:
* Consumes **10 times** less memory than Java-based Selenium server under the same load
* **Small 6 Mb binary** with no external dependencies (no need to install Java)
* **Browser consumption API** working out of the box
* Ability to send browser logs to **centralized log storage** (e.g. to the [ELK-stack](https://logz.io/learn/complete-guide-elk-stack/))
* Fully **isolated** and **reproducible** environment

## Complete Guide & Build Instructions

Complete reference guide can be found at: http://aerokube.com/selenoid/latest/

To build and run from source:
```
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/selenoid_linux_amd64 ./cmd/selenoid
docker compose up -d --build
```
