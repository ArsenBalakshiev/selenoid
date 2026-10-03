from selenium.webdriver.common.by import By
from selenium.webdriver.common.keys import Keys
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.support.ui import WebDriverWait

WIKIPEDIA = "https://www.wikipedia.org"


def test_open_example_domain(driver):
    driver.get("https://example.com")
    assert "Example Domain" in driver.title


def test_wikipedia_search(driver):
    driver.get(WIKIPEDIA)
    search = driver.find_element(By.CSS_SELECTOR, "input[name=search]")
    search.send_keys("Selenium (software)", Keys.ENTER)
    WebDriverWait(driver, 15).until(EC.title_contains("Selenium"))
    assert "Selenium" in driver.title
    assert "wikipedia.org" in driver.current_url


def test_javascript_execution(driver):
    driver.get("https://example.com")
    driver.execute_script("document.title = 'js-test-ok'")
    assert driver.execute_script("return document.title") == "js-test-ok"


def test_screenshot(driver, tmp_path):
    driver.get("https://example.com")
    target = tmp_path / "screen.png"
    driver.save_screenshot(str(target))
    assert target.exists() and target.stat().st_size > 10_000
