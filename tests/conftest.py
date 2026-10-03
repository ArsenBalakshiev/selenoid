import os
import time

import pytest
from selenium import webdriver
from selenium.common.exceptions import WebDriverException
from selenium.webdriver.chrome.options import Options

SELENOID_URL = os.getenv("SELENOID_URL", "http://localhost:4444/wd/hub")
BROWSER = os.getenv("SELENOID_BROWSER", "chrome")
# Geckodriver validates browserVersion against the installed Firefox binary and
# rejects aliases like "latest" ("Requested environment is not available"),
# so the version is only sent when explicitly requested.
BROWSER_VERSION = os.getenv("SELENOID_BROWSER_VERSION", "")
SCREEN_RESOLUTION = os.getenv("SELENOID_SCREEN", "1920x1080x24")
ARTIFACTS_DIR = os.path.join(os.path.dirname(__file__), "artifacts")


@pytest.fixture
def driver():
    options = Options()
    options.set_capability("browserName", BROWSER)
    if BROWSER_VERSION:
        options.set_capability("browserVersion", BROWSER_VERSION)
    options.set_capability(
        "selenoid:options",
        {
            "enableVNC": True,
            "enableVideo": False,
            "screenResolution": SCREEN_RESOLUTION,
            "name": "selenoid-repo-tests",
        },
    )
    # Cold browser containers / proxying hiccup on shared runners may return
    # empty responses for individual session creations, so retries are applied.
    last_error = None
    for attempt in range(5):
        try:
            driver = webdriver.Remote(command_executor=SELENOID_URL, options=options)
            break
        except WebDriverException as e:
            last_error = e
            time.sleep(2 + attempt * 2)
    else:
        raise last_error
    driver.implicitly_wait(5)
    yield driver
    driver.quit()


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
