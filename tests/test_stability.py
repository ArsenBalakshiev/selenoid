# Stability tests: concurrent sessions, queue behaviour under load.

import concurrent.futures

import requests
from conftest import BASE_URL, HUB, BROWSER, selenoid_caps, delete_session, extract_session_id


def test_concurrent_sessions():
    """Three sessions run in parallel and all remain usable."""
    sessions = 3
    with concurrent.futures.ThreadPoolExecutor(max_workers=sessions) as pool:
        futures = [
            pool.submit(
                requests.post,
                HUB + "/session",
                json={"capabilities": {"alwaysMatch": {
                    "browserName": BROWSER,
                    "selenoid:options": selenoid_caps({"name": f"parallel-{i}"}),
                }}},
                timeout=120,
            )
            for i in range(sessions)
        ]
        created = []
        for fut in futures:
            rsp = fut.result()
            assert rsp.status_code == 200, rsp.text
            created.append(extract_session_id(rsp))
    try:
        state = requests.get(BASE_URL + "/status", timeout=10).json()
        assert state["used"] >= sessions

        # All sessions stay responsive in parallel.
        for sid in created:
            rsp = requests.get(f"{HUB}/session/{sid}/url", timeout=30)
            assert rsp.status_code == 200
    finally:
        for sid in created:
            delete_session(sid)
