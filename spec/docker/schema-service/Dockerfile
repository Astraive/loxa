FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app/spec

COPY spec/requirements-service.txt ./
COPY spec/services ./services
COPY spec/generated ./generated
COPY spec/schema ./schema
COPY spec/docker/schema-service/healthcheck.py ./docker/schema-service/healthcheck.py

RUN pip install --no-cache-dir -r requirements-service.txt \
    && adduser --system --uid 10001 --group loza \
    && chown -R loza:loza /app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD python /app/spec/docker/schema-service/healthcheck.py

USER loza

CMD ["python", "-u", "services/schema_service.py", "--host", "0.0.0.0", "--port", "8080"]
