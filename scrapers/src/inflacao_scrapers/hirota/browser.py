"""Let the public storefront initialize an anonymous session, without customer login."""

import time

from .config import API_BASE, CATALOG_PREFIX

HEADER_NAMES = {"authorization", "sessao-id", "domainkey", "organizationid"}


def bootstrap_session(settings) -> dict[str, str]:
    try:
        from botasaurus.browser import browser
    except ImportError as error:
        raise ValueError(
            "Hirota requires the browser extra and Chrome; see scrapers/README.md"
        ) from error
    task = browser(
        headless=True,
        cache=False,
        output=None,
        raise_exception=True,
        max_retry=0,
        create_error_logs=False,
        window_size=(1440, 1000),
        chrome_executable_path=settings.chrome_executable_path,
    )(capture_session)
    return task(settings)


def capture_session(driver, settings):
    headers = {}
    driver.before_request_sent(lambda identity, request, event: capture_headers(request, headers))
    driver.get(settings.shop_url, timeout=settings.timeout_seconds)
    wait_for_session(headers, settings.timeout_seconds)
    text = driver.run_js("return document.body.innerText")
    if "Retirar na loja:" not in text or "Avenida Nazaré, 1299" not in text:
        raise ValueError("Hirota storefront did not select the verified Nazaré pickup context")
    return headers


def capture_headers(request, headers) -> None:
    expected = API_BASE + CATALOG_PREFIX + "produtos/mais_vendidos?"
    if request.method != "GET" or not request.url.startswith(expected):
        return
    headers.update(
        {key: value for key, value in request.headers.items() if key.lower() in HEADER_NAMES}
    )


def wait_for_session(headers, timeout) -> None:
    deadline = time.monotonic() + timeout
    while not headers and time.monotonic() < deadline:
        time.sleep(0.25)
    if not headers:
        raise ValueError("Hirota anonymous storefront session unavailable; collection stopped")
