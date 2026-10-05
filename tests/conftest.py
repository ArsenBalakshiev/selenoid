import os
import time

import pytest
import requests
from selenium import webdriver
from selenium.common.exceptions import WebDriverException
from selenium.webdriver.chrome.options import Options

SELENOID_URL = os.getenv("SELENOID_URL", "http://localhost:4444/wd/hub")
# API base URL: SELENOID_URL points at /wd/hub, e.g. http://selenoid:4444/wd/hub
BASE_URL = SELENOID_URL.rsplit("/wd/hub", 1)[0].rstrip("/")
HUB = BASE_URL + "/wd/hub"
# Comma separated browser list; every driver-based test is repeated per browser.
BROWSERS = [
    b.strip()
    for b in os.getenv("SELENOID_BROWSERS", os.getenv("SELENOID_BROWSER", "chrome")).split(",")
    if b.strip()
]
BROWSER = BROWSERS[0]
BROWSER_VERSION = os.getenv("SELENOID_BROWSER_VERSION", "")
SCREEN_RESOLUTION = os.getenv("SELENOID_SCREEN", "1920x1080x24")
ARTIFACTS_DIR = os.path.join(os.path.dirname(__file__), "artifacts")


def selenoid_caps(extra=None):
    caps = {
        "enableVNC": True,
        "screenResolution": SCREEN_RESOLUTION,
        "name": "selenoid-repo-tests",
    }
    caps.update(extra or {})
    return caps


def create_session(capabilities=None, browser=None, version=None, extra=None, no_wait=False):
    """Create a session through the WebDriver API and return the response."""
    browser = browser or BROWSER
    caps = selenoid_caps(extra)
    if capabilities:
        caps.update(capabilities)
    always_match = {"browserName": browser}
    if version or BROWSER_VERSION:
        always_match["browserVersion"] = version or BROWSER_VERSION
    always_match["selenoid:options"] = caps
    headers = {"X-Selenoid-No-Wait": "1"} if no_wait else {}
    return requests.post(HUB + "/session", json={"capabilities": {"alwaysMatch": always_match}}, headers=headers, timeout=120)


def delete_session(sid):
    return requests.delete(f"{HUB}/session/{sid}", timeout=60)


def extract_session_id(rsp):
    """WebDriver sends sessionId at the root (JSONWire) or inside value (W3C)."""
    body = rsp.json()
    if "sessionId" in body:
        return body["sessionId"]
    return body["value"]["sessionId"]


@pytest.fixture(params=BROWSERS)
def driver(request):
    options = Options()
    options.set_capability("browserName", request.param)
    if BROWSER_VERSION:
        options.set_capability("browserVersion", BROWSER_VERSION)
    options.set_capability("selenoid:options", selenoid_caps({"enableVideo": False}))
    # Cold browser containers / proxying hiccup on shared runners may return
    # empty responses for individual session creations, so retries are applied.
    last_error = None
    for attempt in range(5):
        try:
            drv = webdriver.Remote(command_executor=SELENOID_URL, options=options)
            break
        except WebDriverException as e:
            last_error = e
            time.sleep(2 + attempt * 2)
    else:
        raise last_error
    drv.implicitly_wait(5)
    yield drv
    drv.quit()


@pytest.hookimpl(hookwrapper=True, tryfirst=True)
def pytest_runtest_makereport(item, call):
    outcome = yield
    report = outcome.get_result()
    if report.when == "call" and report.failed:
        driver = item.funcargs.get("driver")
        if driver is None:
            return
        os.makedirs(ARTIFACTS_DIR, exist_ok=True)
        name = f"{item.name}-{int(time.time())}.png"
        try:
            driver.get_screenshot_as_file(os.path.join(ARTIFACTS_DIR, name))
        except Exception:
            pass
