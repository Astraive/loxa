Containerized deployment and Kubernetes rollout

This repository includes production-ready container assets for:
- schema-service: serves generated/contract/loxa-contract.json and spec schemas.
- stager: continuously drains the ingest queue and writes staged partition files.

Docker

Build images from the loxa-spec context:

  docker build -f loxa-spec/docker/schema-service/Dockerfile -t ghcr.io/astraive/loxa-schema-service:latest loxa-spec
  docker build -f loxa-spec/docker/stager/Dockerfile -t ghcr.io/astraive/loxa-stager:latest loxa-spec

Run local compose stack:

  docker compose up --build

The compose stack configures the stager for Kafka by default:
- STAGER_QUEUE_BACKEND=kafka
- KAFKA_BOOTSTRAP_SERVERS=kafka:9092
- KAFKA_TOPIC=loxa-events
- KAFKA_GROUP_ID=loxa-stager
- KAFKA_DLQ_TOPIC=loxa-events.dlq

Kubernetes manifests (raw YAML)

Manifests are in loxa-spec/deploy/k8s and include ConfigMap + Deployment (+ Service for schema-service),
resource requests/limits, and readiness/liveness probes. These are thin examples that should stay aligned
with the Helm chart.

  kubectl apply -k loxa-spec/deploy/k8s

Optional secrets referenced by Deployments:
- loxa-schema-service-secrets
- loxa-stager-secrets

Helm chart

Canonical chart path: loxa-spec/charts/loxa

Install or upgrade:

  helm upgrade --install loxa .\loxa-spec\charts\loxa --namespace loxa --create-namespace

Override image tags during rollout:

  helm upgrade --install loxa .\loxa-spec\charts\loxa --namespace loxa --set schemaService.image.tag=v1.2.3 --set stager.image.tag=v1.2.3

Render templates without installing:

  helm template loxa .\loxa-spec\charts\loxa --namespace loxa

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
      kafka_topic="loxa-events",
      kafka_group_id="loxa-stager",
      kafka_dlq_topic="loxa-events.dlq",
  )
