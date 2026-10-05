# Video recording end-to-end: enableVideo capability + /video API.
#
# Default target is the official selenoid/vnc:firefox image. The custom
# chrome/yandex *-vnc-fixed images (Xvfb with -listen tcp + live x11vnc) also
# work: point SELENOID_VIDEO_BROWSER / SELENOID_VIDEO_BROWSER_VERSION there.

import time

import requests
from conftest import BASE_URL, delete_session, extract_session_id
from test_helpers import VIDEO_BROWSER, VIDEO_BROWSER_VERSION


def _selenium_url():
    return BASE_URL + "/wd/hub/session"


def _video_listing():
    rsp = requests.get(BASE_URL + "/video/?json", timeout=15)
    assert rsp.status_code == 200
    return [] if rsp.text.strip() in ("null", "") else rsp.json()


def _download(name):
    return requests.get(BASE_URL + "/video/" + name, timeout=60)


def _delete(name):
    rsp = requests.delete(BASE_URL + "/video/" + name, timeout=30)
    assert rsp.status_code == 200, rsp.text


def test_video_recorded_and_served():
    # Fresh naming space: remove leftovers first.
    for existing in _video_listing():
        _delete(existing)

    always_match = {"browserName": VIDEO_BROWSER}
    if VIDEO_BROWSER_VERSION:
        always_match["browserVersion"] = VIDEO_BROWSER_VERSION
    always_match["selenoid:options"] = {"enableVideo": True, "enableVNC": True, "name": "e2e-video-test"}

    rsp = requests.post(
        BASE_URL + "/wd/hub/session",
        json={"capabilities": {"alwaysMatch": always_match}},
        timeout=120,
    )
    assert rsp.status_code == 200, rsp.text
    sid = extract_session_id(rsp)

    # Navigate and keep the session alive for a while: ffmpeg needs several
    # seconds of display activity to produce a real MP4 payload. The file
    # only becomes available after the recorder container is finalized.
    try:
        rsp = requests.post(
            BASE_URL + "/wd/hub/session/" + sid + "/url",
            json={"url": "https://example.com"},
            timeout=60,
        )
        assert rsp.status_code == 200, rsp.text
        # ffmpeg starts capturing a few seconds in; slightly longer session
        # guarantees a real payload rather than an empty container.
        time.sleep(45)
    finally:
        assert delete_session(sid).status_code in (200, 204)

    # The temporary file is created at session start; the FINAL recording
    # (named <sessionId>.mp4) only appears once the session is deleted.
    name = sid + ".mp4"
    deadline = time.time() + 90
    while time.time() < deadline:
        if name in _video_listing():
            break
        time.sleep(2)
    else:
        raise AssertionError(f"recorded video {name} did not appear within 90s (files: {_video_listing()})")

    rsp = _download(name)
    assert rsp.status_code == 200
    assert len(rsp.content) > 9_000, f"video file too small: {len(rsp.content)} bytes"

    _delete(name)
    time.sleep(1)
    assert name not in _video_listing()
