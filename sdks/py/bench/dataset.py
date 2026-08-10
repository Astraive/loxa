"""Deterministic benchmark dataset generator for the Persistent Context Engine.

Generates the L2 public benchmark defaults:
- 12 services, 7 days, ~17k background events
- 30 deployments, 8 topology mutations (renames)
- 24 train + 10 eval incidents, 5 recurring families
- Seeded RNG for reproducibility
"""

from __future__ import annotations

import json
import random
from datetime import datetime, timedelta, timezone


# L2 defaults
N_SERVICES = 12
N_DAYS = 7
N_DEPLOYS = 30
N_TOPOLOGY_MUTATIONS = 8
N_TRAIN_INCIDENTS = 24
N_EVAL_INCIDENTS = 10
N_RECURRING_FAMILIES = 5
BACKGROUND_EVENTS_PER_SERVICE_DAY = 200

SERVICE_NAMES = [
    "checkout-api", "payments-svc", "inventory-svc", "user-svc",
    "notification-svc", "shipping-svc", "catalog-svc", "auth-svc",
    "search-svc", "analytics-svc", "gateway-svc", "cache-svc",
]

INCIDENT_FAMILIES = [
    {"name": "deploy-latency", "symptom": "latency_spike", "kind_sequence": ["deploy", "metric", "log"]},
    {"name": "error-cascade", "symptom": "error_rate", "kind_sequence": ["log", "metric", "trace"]},
    {"name": "timeout-chain", "symptom": "timeout", "kind_sequence": ["metric", "log", "trace"]},
    {"name": "resource-exhaustion", "symptom": "memory_leak", "kind_sequence": ["metric", "metric", "log"]},
    {"name": "deploy-failure", "symptom": "deployment_fail", "kind_sequence": ["deploy", "log", "metric"]},
]


def generate_dataset(seed: int = 42) -> dict:
    """Generate the full benchmark dataset.

    Returns:
        dict with keys: train_events, eval_events, eval_signals, topology_mutations
    """
    rng = random.Random(seed)
    base_time = datetime(2026, 5, 1, 0, 0, 0, tzinfo=timezone.utc)

    all_events: list[dict] = []
    topology_mutations: list[dict] = []
    eval_signals: list[dict] = []

    # Generate background events
    for day in range(N_DAYS):
        for svc_idx, svc in enumerate(SERVICE_NAMES):
            n_events = rng.randint(
                BACKGROUND_EVENTS_PER_SERVICE_DAY - 50,
                BACKGROUND_EVENTS_PER_SERVICE_DAY + 50,
            )
            for _ in range(n_events):
                ts = base_time + timedelta(
                    days=day,
                    hours=rng.randint(0, 23),
                    minutes=rng.randint(0, 59),
                    seconds=rng.randint(0, 59),
                )
                kind = rng.choices(
                    ["log", "metric", "trace", "loza_event"],
                    weights=[40, 30, 20, 10],
                )[0]
                event = _make_event(ts, kind, svc, rng)
                all_events.append(event)

    # Generate deployments
    deploy_times = []
    for i in range(N_DEPLOYS):
        day = rng.randint(0, N_DAYS - 1)
        hour = rng.randint(8, 18)
        ts = base_time + timedelta(days=day, hours=hour, minutes=rng.randint(0, 59))
        svc = rng.choice(SERVICE_NAMES)
        version = f"v{rng.randint(1, 5)}.{rng.randint(0, 20)}.{rng.randint(0, 9)}"
        event = {
            "ts": ts.isoformat(),
            "kind": "deploy",
            "service": svc,
            "version": version,
            "actor": "ci",
        }
        all_events.append(event)
        deploy_times.append((ts, svc, version))

    # Generate topology mutations (renames)
    services_to_rename = rng.sample(SERVICE_NAMES, min(N_TOPOLOGY_MUTATIONS, len(SERVICE_NAMES)))
    for i, svc in enumerate(services_to_rename):
        day = rng.randint(2, N_DAYS - 1)
        ts = base_time + timedelta(days=day, hours=rng.randint(10, 16))
        new_name = f"{svc}-v2" if i % 2 == 0 else svc.replace("-svc", "-service")
        mutation = {
            "ts": ts.isoformat(),
            "kind": "topology",
            "change": "rename",
            "from": svc,
            "to": new_name,
        }
        all_events.append(mutation)
        topology_mutations.append(mutation)

    # Generate train incidents
    train_incidents = _generate_incidents(
        rng, base_time, N_TRAIN_INCIDENTS, deploy_times, "train",
    )
    for inc in train_incidents:
        all_events.extend(inc["events"])
        if inc.get("remediation"):
            all_events.append(inc["remediation"])

    # Generate eval incidents (held-out)
    eval_incidents = _generate_incidents(
        rng, base_time + timedelta(days=N_DAYS // 2), N_EVAL_INCIDENTS, deploy_times, "eval",
    )
    for inc in eval_incidents:
        all_events.extend(inc["events"])
        eval_signals.append(inc["signal"])

    # Sort all events by timestamp
    all_events.sort(key=lambda e: e["ts"])

    # Split train/eval by time boundary
    split_time = (base_time + timedelta(days=N_DAYS - 2)).isoformat()
    train_events = [e for e in all_events if e["ts"] < split_time]
    eval_events = [e for e in all_events if e["ts"] >= split_time]

    return {
        "train_events": train_events,
        "eval_events": eval_events,
        "eval_signals": eval_signals,
        "topology_mutations": topology_mutations,
    }


def _make_event(ts: datetime, kind: str, service: str, rng: random.Random) -> dict:
    event: dict = {
        "ts": ts.isoformat(),
        "kind": kind,
        "service": service,
    }
    if kind == "log":
        level = rng.choices(["info", "warn", "error"], weights=[70, 20, 10])[0]
        event["level"] = level
        event["msg"] = f"Sample {level} log from {service}"
        if level == "error":
            event["msg"] = f"Error in {service}: connection timeout"
    elif kind == "metric":
        event["name"] = rng.choice(["latency_p99_ms", "error_rate", "cpu_usage", "memory_mb"])
        event["value"] = rng.uniform(10, 500)
    elif kind == "trace":
        event["trace_id"] = f"trace-{rng.randint(1000, 9999)}"
        event["spans"] = [{"svc": service, "dur_ms": rng.randint(5, 500)}]
    return event


def _generate_incidents(
    rng: random.Random,
    base_time: datetime,
    count: int,
    deploy_times: list[tuple],
    phase: str,
) -> list[dict]:
    incidents = []
    for i in range(count):
        family = INCIDENT_FAMILIES[i % len(INCIDENT_FAMILIES)]
        day = rng.randint(0, 6)
        ts = base_time + timedelta(days=day, hours=rng.randint(8, 22))
        svc = rng.choice(SERVICE_NAMES)
        incident_id = f"INC-{100 + len(incidents)}"

        # Signal
        signal = {
            "ts": ts.isoformat(),
            "kind": "incident_signal",
            "incident_id": incident_id,
            "service": svc,
            "trigger": f"alert:{svc}/{family['symptom']}>threshold",
        }

        # Events leading up to incident
        events = []
        event_ts = ts - timedelta(minutes=rng.randint(2, 15))
        for kind in family["kind_sequence"]:
            event = _make_event(event_ts, kind, svc, rng)
            event["incident_id"] = incident_id
            if kind == "metric":
                event["value"] = rng.uniform(500, 5000)  # anomalous value
            events.append(event)
            event_ts += timedelta(seconds=rng.randint(10, 120))

        events.append(signal)

        # Remediation
        remediation = {
            "ts": (ts + timedelta(minutes=rng.randint(5, 30))).isoformat(),
            "kind": "remediation",
            "incident_id": incident_id,
            "action": rng.choice(["rollback", "restart", "scale-up", "failover"]),
            "target": svc,
            "outcome": rng.choices(["success", "failed"], weights=[70, 30])[0],
        }

        incidents.append({
            "incident_id": incident_id,
            "signal": signal,
            "events": events,
            "remediation": remediation,
            "family": family["name"],
        })

    return incidents


def save_dataset_jsonl(dataset: dict, prefix: str = "bench_") -> None:
    """Save dataset to JSONL files."""
    for key in ["train_events", "eval_events"]:
        filename = f"{prefix}{key}.jsonl"
        with open(filename, "w") as f:
            for event in dataset[key]:
                f.write(json.dumps(event) + "\n")
        print(f"Wrote {len(dataset[key])} events to {filename}")


if __name__ == "__main__":
    ds = generate_dataset()
    print(f"Generated: {len(ds['train_events'])} train, {len(ds['eval_events'])} eval events")
    print(f"Topology mutations: {len(ds['topology_mutations'])}")
    print(f"Eval signals: {len(ds['eval_signals'])}")
