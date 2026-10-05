# Hub HTTP API end-to-end tests: no browser required, crafted with raw HTTP requests.

import pytest
import requests
from conftest import BASE_URL, HUB, create_session, delete_session, extract_session_id


def test_status_lists_browsers():
    rsp = requests.get(BASE_URL + "/status", timeout=10)
    assert rsp.status_code == 200
    state = rsp.json()
    # total reflects the configured session limit (5 in compose)
    assert state["total"] >= 1
    assert "chrome" in state["browsers"]
    assert "firefox" in state["browsers"]


def test_ping():
    rsp = requests.get(BASE_URL + "/ping", timeout=10)
    assert rsp.status_code == 200
    data = rsp.json()
    assert data["version"]
    assert data["uptime"]
    assert "numRequests" in data
    assert "lastReloadTime" in data


def test_welcome_screen():
    for path in ("/", "/wd/hub"):
        rsp = requests.get(BASE_URL + path, timeout=10)
        assert rsp.status_code == 200
        assert "You are using Selenoid" in rsp.text


def test_unknown_browser_returns_400():
    rsp = requests.post(
        HUB + "/session",
        json={"capabilities": {"alwaysMatch": {"browserName": "no-such-browser"}}},
        timeout=60,
    )
    assert rsp.status_code == 400
    assert "requested environment is not available" in rsp.json()["value"]["message"]


def test_bad_json_returns_400():
    rsp = requests.post(HUB + "/session", data="&^%", timeout=60)
    assert rsp.status_code == 400
    body = rsp.json()["value"]
    assert body["error"] == "invalid argument"


def test_get_method_rejected_on_session_creation():
    rsp = requests.get(HUB + "/session", timeout=10)
    assert rsp.status_code == 405


def test_unknown_session_proxy_returns_404():
    rsp = requests.get(HUB + "/session/definitely-not-a-session/url", timeout=10)
    assert rsp.status_code == 404
    assert rsp.json()["value"]["error"] == "invalid session id"


def test_unknown_session_delete_returns_404():
    rsp = requests.delete(HUB + "/session/definitely-not-a-session", timeout=10)
    assert rsp.status_code == 404


def test_session_lifecycle():
    # Create.
    rsp = create_session({"enableVideo": False})
    assert rsp.status_code == 200, rsp.text
    sid = extract_session_id(rsp)

    try:
        # Proxy through the session: navigate and read the url back.
        rsp = requests.post(
            f"{HUB}/session/{sid}/url",
            json={"url": "https://example.com"},
            timeout=60,
        )
        assert rsp.status_code == 200, rsp.text
        rsp = requests.get(f"{HUB}/session/{sid}/url", timeout=30)
        assert rsp.status_code == 200
        assert "example.com" in rsp.json()["value"]

        # Session is visible in state.
        rsp = requests.get(BASE_URL + "/status", timeout=10)
        assert rsp.status_code == 200
        assert rsp.json()["used"] >= 1
    finally:
        rsp = delete_session(sid)
        assert rsp.status_code in (200, 204)

    # After deletion the session is gone: proxy must answer 404.
    rsp = requests.get(f"{HUB}/session/{sid}/url", timeout=30)
    assert rsp.status_code == 404


def test_session_vnc_visible_in_status():
    rsp = create_session({"enableVideo": False})
    assert rsp.status_code == 200, rsp.text
    sid = extract_session_id(rsp)
    try:
        rsp = requests.get(BASE_URL + "/status", timeout=10)
        state = rsp.json()
        entries = [
            s
            for v in state["browsers"].values()
            for q in v.values()
            for s in q.values()
            for s in s["sessions"]
        ]
        assert any(s["id"] == sid and s["vnc"] for s in entries), f"session {sid} not found with VNC enabled"
    finally:
        delete_session(sid)


def test_queue_full_no_wait_rejects_immediately():
    """With the limit reached, X-Selenoid-No-Wait request fails fast."""
    created = []
    try:
        for _ in range(5):  # compose sets -limit 5
            rsp = create_session({"enableVNC": False, "enableVideo": False})
            assert rsp.status_code in (200, 500, 429)
            if rsp.status_code == 200:
                created.append(extract_session_id(rsp))
        if len(created) < 5:
            pytest.skip(f"only {len(created)} sessions created - not enough capacity to fill the queue")
        rsp = requests.post(
            HUB + "/session",
            json={"capabilities": {"alwaysMatch": {"browserName": "chrome"}}},
            headers={"X-Selenoid-No-Wait": "1"},
            timeout=30,
        )
        assert rsp.status_code == 500, f"expected fast rejection, got {rsp.status_code}"
        assert rsp.json()["value"]["message"] == "Too Many Requests"
    finally:
        for sid in created:
            delete_session(sid)
