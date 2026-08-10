Containerized deployment and Kubernetes rollout

This repository includes production-ready container assets for:
- schema-service: serves generated/contract/loza-contract.json and spec schemas.
- stager: continuously drains the ingest queue and writes staged partition files.

Docker

Build images from the spec context:

  docker build -f spec/docker/schema-service/Dockerfile -t ghcr.io/astraive/loza-schema-service:latest spec
  docker build -f spec/docker/stager/Dockerfile -t ghcr.io/astraive/loza-stager:latest spec

Run local compose stack:

  docker compose up --build

The compose stack configures the stager for Kafka by default:
- STAGER_QUEUE_BACKEND=kafka
- KAFKA_BOOTSTRAP_SERVERS=kafka:9092
- KAFKA_TOPIC=loza-events
- KAFKA_GROUP_ID=loza-stager
- KAFKA_DLQ_TOPIC=loza-events.dlq

Kubernetes manifests (raw YAML)

Manifests are in spec/deploy/k8s and include ConfigMap + Deployment (+ Service for schema-service),
resource requests/limits, and readiness/liveness probes. These are thin examples that should stay aligned
with the Helm chart.

  kubectl apply -k spec/deploy/k8s

Optional secrets referenced by Deployments:
- loza-schema-service-secrets
- loza-stager-secrets

Helm chart

Canonical chart path: spec/charts/loza

Install or upgrade:

  helm upgrade --install loza .\spec\charts\loza --namespace loza --create-namespace

Override image tags during rollout:

  helm upgrade --install loza .\spec\charts\loza --namespace loza --set schemaService.image.tag=v0.0.1 --set stager.image.tag=v0.0.1

Render templates without installing:

  helm template loza .\spec\charts\loza --namespace loza

Kafka-backed stager path (Python)

  pip install confluent-kafka

  from pathlib import Path
  from services.kafka_adapter import RetryPolicy
  from services.stager import run_worker

  run_worker(
      backend="kafka",
      queue_dir="./tmp_ingest_queue",
      staging_dir=Path("./tmp_staging"),
      batch_size=100,
      compress=True,
      retry_policy=RetryPolicy(max_attempts=5, base_delay_seconds=0.2),
      kafka_brokers="localhost:9092",
      kafka_topic="loza-events",
      kafka_group_id="loza-stager",
      kafka_dlq_topic="loza-events.dlq",
  )
