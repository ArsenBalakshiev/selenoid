# Session logs end-to-end: enableLog capability + /logs API.

import time

import requests
from conftest import BASE_URL, delete_session, extract_session_id


def _logs_listing():
    rsp = requests.get(BASE_URL + "/logs/?json", timeout=15)
    assert rsp.status_code == 200
    return [] if rsp.text.strip() in ("null", "") else rsp.json()


def test_session_log_saved_and_served():
    rsp = requests.post(
        BASE_URL + "/wd/hub/session",
        json={"capabilities": {"alwaysMatch": {
            "browserName": "chrome",
            "selenoid:options": {"enableLog": True, "name": "e2e-log-test"},
        }}},
        timeout=120,
    )
    assert rsp.status_code == 200, rsp.text
    sid = extract_session_id(rsp)

    try:
        requests.get(
            BASE_URL + "/wd/hub/session/" + sid + "/url",
            params={"url": "https://example.com"},
            timeout=60,
        )
    finally:
        assert delete_session(sid).status_code in (200, 204)

    # Logs are renamed into place when the session is deleted and uploaded
    # asynchronously; poll for the file.
    name = sid + ".log"
    deadline = time.time() + 30
    while time.time() < deadline:
        if name in _logs_listing():
            break
        time.sleep(2)
    else:
        raise AssertionError(f"log file {name} did not appear within 30s (files: {_logs_listing()})")

    rsp = requests.get(BASE_URL + "/logs/" + name, timeout=30)
    assert rsp.status_code == 200
    assert len(rsp.content) > 0

    rsp = requests.delete(BASE_URL + "/logs/" + name, timeout=30)
    assert rsp.status_code == 200
