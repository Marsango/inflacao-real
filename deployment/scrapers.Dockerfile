FROM python:3.12.12-slim-bookworm@sha256:593bd06efe90efa80dc4eee3948be7c0fde4134606dd40d8dd8dbcade98e669c
ENV PYTHONUNBUFFERED=1 PYTHONDONTWRITEBYTECODE=1
WORKDIR /app
COPY deployment/requirements.lock /tmp/requirements.lock
RUN pip install --no-cache-dir -r /tmp/requirements.lock setuptools==80.9.0
COPY scrapers/ /source/
RUN pip install --no-cache-dir --no-deps --no-build-isolation /source && rm -rf /source && mkdir /app/data && chown 10001:10001 /app/data
USER 10001:10001
CMD ["inflacao-scraper-schedule", "--config", "/app/config.toml", "--at", "09:00", "--timezone", "America/Sao_Paulo"]
