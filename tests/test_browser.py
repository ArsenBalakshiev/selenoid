# Browser functionality tests running through the Selenoid proxy.
# Each test is repeated for every browser in SELENOID_BROWSERS (default: chrome).

from selenium.webdriver.common.by import By
from selenium.webdriver.common.keys import Keys
from selenium.webdriver.common.window import WindowTypes
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.support.ui import WebDriverWait

EXAMPLE = "https://example.com"


def test_open_example_domain(driver):
    driver.get(EXAMPLE)
    assert "Example Domain" in driver.title


def test_wikipedia_search(driver):
    driver.get("https://www.wikipedia.org")
    search = driver.find_element(By.CSS_SELECTOR, "input[name=search]")
    search.send_keys("Selenium (software)", Keys.ENTER)
    WebDriverWait(driver, 15).until(EC.title_contains("Selenium"))
    assert "Selenium" in driver.title
    assert "wikipedia.org" in driver.current_url


def test_javascript_execution(driver):
    driver.get(EXAMPLE)
    driver.execute_script("document.title = 'js-test-ok'")
    assert driver.execute_script("return document.title") == "js-test-ok"


def test_screenshot(driver, tmp_path):
    driver.get(EXAMPLE)
    target = tmp_path / "screen.png"
    driver.save_screenshot(str(target))
    assert target.exists() and target.stat().st_size > 10_000


def test_cookie_roundtrip(driver):
    driver.get(EXAMPLE)
    driver.add_cookie({"name": "sel-test", "value": "cookie-ok"})
    assert driver.get_cookie("sel-test")["value"] == "cookie-ok"
    driver.delete_all_cookies()
    assert driver.get_cookie("sel-test") is None


def test_navigation_back_forward(driver):
    driver.get(EXAMPLE)
    first_url = driver.current_url
    driver.get("https://www.w3.org")
    driver.back()
    assert driver.current_url == first_url
    driver.forward()
    assert "w3.org" in driver.current_url


def test_window_resize(driver):
    driver.get(EXAMPLE)
    driver.set_window_size(800, 600)
    size = driver.get_window_size()
    assert size["width"] == 800 and size["height"] == 600


def test_window_open_switch_close(driver):
    driver.get(EXAMPLE)
    original = driver.current_window_handle
    driver.switch_to.new_window(WindowTypes.TAB)
    assert len(driver.window_handles) == 2
    driver.get(EXAMPLE)
    driver.switch_to.window(original)
    assert len(driver.window_handles) == 2
    driver.close()
    driver.switch_to.window(driver.window_handles[0])


def test_current_url_and_source(driver):
    driver.get(EXAMPLE)
    assert "http" in driver.current_url
    assert "Example Domain" in driver.page_source