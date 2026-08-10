FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    STAGER_QUEUE_BACKEND=kafka \
    STAGER_LOOP=true \
    STAGER_POLL_INTERVAL_SECONDS=2 \
    STAGER_HEARTBEAT_FILE=/app/health/stager.heartbeat

WORKDIR /app/spec

COPY spec/requirements-service.txt ./
COPY spec/services ./services

RUN pip install --no-cache-dir -r requirements-service.txt \
    && adduser --system --uid 10001 --group loza \
    && mkdir -p /app/spec/tmp_ingest_queue /app/spec/tmp_staging /app/health \
    && chown -R loza:loza /app

VOLUME ["/app/spec/tmp_ingest_queue", "/app/spec/tmp_staging"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD python -c "import os,time,pathlib,sys; p=pathlib.Path(os.environ.get('STAGER_HEARTBEAT_FILE','/app/health/stager.heartbeat')); max_age=float(os.environ.get('STAGER_HEARTBEAT_MAX_AGE_SECONDS','120')); sys.exit(0 if p.exists() and (time.time()-p.stat().st_mtime) <= max_age else 1)"

USER loza

CMD ["python", "-u", "-m", "services.stager"]
