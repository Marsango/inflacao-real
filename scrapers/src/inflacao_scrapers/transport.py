"""Bounded HTTP attempts, retry guidance, and conservative request pacing."""

import random
import time
from dataclasses import dataclass
from datetime import UTC, datetime
from email.utils import parsedate_to_datetime

import httpx


class RequestFailure(RuntimeError):
    pass


def retry_after(response: httpx.Response | None) -> float:
    if response is None:
        return 0
    header = response.headers.get("Retry-After", "")
    if header.isascii() and header.isdigit():
        return float(header)
    try:
        remaining = parsedate_to_datetime(header) - datetime.now(UTC)
        return remaining.total_seconds()
    except (ValueError, TypeError, OverflowError):
        return 0


def transient(response: httpx.Response | None) -> bool:
    return response is None or response.status_code in (429, 500, 502, 503, 504)


def send_safely(client: httpx.Client, method: str, url: str, arguments: dict):
    try:
        return client.request(method, url, follow_redirects=False, **arguments)
    except httpx.TransportError:
        # Request/exception representations may contain credentials.
        return None


@dataclass
class RetryPolicy:
    attempts: int
    base: float
    maximum: float

    def wait(self, response: httpx.Response | None, attempt: int, sleep) -> None:
        if attempt + 1 == self.attempts:
            raise RequestFailure(self.exhausted_message(response))
        guidance = retry_after(response)
        if guidance > self.maximum:
            raise RequestFailure("Retry-After exceeds this run's retry budget; retry later")
        delay = min(self.maximum, self.base * 2**attempt) + random.uniform(0, 0.25)
        sleep(max(delay, guidance))

    def exhausted_message(self, response: httpx.Response | None) -> str:
        status = "network error"
        if response is not None:
            status = f"HTTP {response.status_code}"
        return f"{status}; exhausted {self.attempts} attempts"


class Requester:
    def __init__(
        self,
        client: httpx.Client,
        *,
        attempts=3,
        interval=1,
        retry_base=1,
        retry_max=60,
        sleep=time.sleep,
        clock=time.monotonic,
    ):
        self.client = client
        self.policy = RetryPolicy(attempts, retry_base, retry_max)
        self.interval, self.sleep, self.clock = interval, sleep, clock
        self.last_request: float | None = None

    def request(self, method: str, url: str, **arguments) -> httpx.Response:
        return self.attempt(method, url, arguments, 0)

    def attempt(self, method: str, url: str, arguments: dict, attempt: int) -> httpx.Response:
        self.pace()
        response = send_safely(self.client, method, url, arguments)
        if not transient(response):
            return response
        self.policy.wait(response, attempt, self.sleep)
        return self.attempt(method, url, arguments, attempt + 1)

    def pace(self) -> None:
        if self.last_request is not None:
            elapsed = self.clock() - self.last_request
            self.sleep(max(0, self.interval - elapsed))
        self.last_request = self.clock()
