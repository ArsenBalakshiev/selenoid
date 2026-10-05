# Shared settings for e2e tests (no tests in here; collected as module only).

import os

# Video needs a working X server reachable over X11 TCP by the recorder
# (and, for VNC viewing, a live x11vnc). The custom chrome/yandex *-vnc-fixed
# images provide both; default video source is firefox, which has always
# worked. Set SELENOID_VIDEO_BROWSER / SELENOID_VIDEO_BROWSER_VERSION to
# switch target, e.g. chrome + latest-vnc.
VIDEO_BROWSER = os.getenv("SELENOID_VIDEO_BROWSER", "firefox")
VIDEO_BROWSER_VERSION = os.getenv("SELENOID_VIDEO_BROWSER_VERSION", "")
