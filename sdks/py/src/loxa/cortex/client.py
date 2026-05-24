"""Cortex HTTP client for incident intelligence operations."""

from __future__ import annotations

import json
import urllib.request
from urllib.error import HTTPError, URLError

from .models import GraphView, IncidentContext, Remediation, RemediationFeedback


class CortexClient:
    """Client for the Cortex incident intelligence API.

    Provides methods for incident reconstruction, graph queries,
    similar incident lookup, and remediation recording.

    Example::

        from loxa.cortex import CortexClient

        cortex = CortexClient("http://localhost:8080")
        if cortex.health():
            ctx = cortex.reconstruct("inc-123")
            print(ctx.causal_chain)
    """

    def __init__(
        self,
        endpoint: str = "http://localhost:8080",
        api_key: str = "",
        auth_header: str = "Authorization",
        timeout: float = 10.0,
    ) -> None:
        self.endpoint = endpoint.rstrip("/")
        self.api_key = api_key
        self.auth_header = auth_header
        self.timeout = timeout

    def _headers(self) -> dict[str, str]:
        headers = {"content-type": "application/json"}
        if self.api_key:
            if self.auth_header.lower() == "authorization":
                headers[self.auth_header] = f"Bearer {self.api_key}"
            else:
                headers[self.auth_header] = self.api_key
        return headers

    def _get(self, path: str) -> dict:
        url = f"{self.endpoint}{path}"
        req = urllib.request.Request(url, headers=self._headers(), method="GET")
        with urllib.request.urlopen(req, timeout=self.timeout) as resp:
            return json.loads(resp.read())

    def _post(self, path: str, body: dict | None = None) -> dict:
        url = f"{self.endpoint}{path}"
        data = json.dumps(body or {}).encode("utf-8")
        req = urllib.request.Request(url, data=data, headers=self._headers(), method="POST")
        with urllib.request.urlopen(req, timeout=self.timeout) as resp:
            raw = resp.read()
            if raw:
                return json.loads(raw)
            return {}

    def health(self) -> bool:
        """Check if cortex is healthy."""
        try:
            resp = self._get("/healthz")
            return resp.get("status") == "ok"
        except (URLError, HTTPError, OSError):
            return False

    def ready(self) -> bool:
        """Check if cortex is ready to accept requests."""
        try:
            resp = self._get("/readyz")
            return resp.get("status") == "ok" or resp.get("ready") is True
        except (URLError, HTTPError, OSError):
            return False

    def metrics(self) -> str:
        """Fetch Prometheus metrics from cortex."""
        url = f"{self.endpoint}/metrics"
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=self.timeout) as resp:
            return resp.read().decode("utf-8")

    def reconstruct(
        self,
        incident_id: str,
        mode: str = "fast",
    ) -> IncidentContext:
        """Reconstruct an incident timeline with root cause analysis.

        Args:
            incident_id: The incident to reconstruct.
            mode: Reconstruction mode - "fast" (pattern match) or "deep" (full causal analysis).
        """
        resp = self._post("/reconstruct", {"incident_id": incident_id, "mode": mode})
        return IncidentContext(
            incident_id=resp.get("incident_id", incident_id),
            timestamp=resp.get("timestamp", ""),
            causal_chain=resp.get("causal_chain", []),
            related_services=resp.get("related_services", []),
            symptoms=resp.get("symptoms", []),
            suggested_actions=resp.get("suggested_actions", []),
            confidence=resp.get("confidence", 0.0),
            similar_incidents=resp.get("similar_incidents"),
            explain=resp.get("explain", ""),
            explain_report=resp.get("explain_report"),
        )

    def reconstruct_incident(
        self,
        incident_id: str,
        mode: str = "fast",
    ) -> IncidentContext:
        """Reconstruct an incident using the URL-param variant."""
        resp = self._post(f"/incidents/{incident_id}/reconstruct", {"mode": mode})
        return IncidentContext(
            incident_id=resp.get("incident_id", incident_id),
            timestamp=resp.get("timestamp", ""),
            causal_chain=resp.get("causal_chain", []),
            related_services=resp.get("related_services", []),
            symptoms=resp.get("symptoms", []),
            suggested_actions=resp.get("suggested_actions", []),
            confidence=resp.get("confidence", 0.0),
            similar_incidents=resp.get("similar_incidents"),
            explain=resp.get("explain", ""),
            explain_report=resp.get("explain_report"),
        )

    def service_graph(
        self,
        service: str,
        depth: int = 3,
    ) -> GraphView:
        """Fetch the dependency graph for a service."""
        resp = self._get(f"/graph/service/{service}?depth={depth}")
        return GraphView(
            nodes=resp.get("nodes", []),
            edges=resp.get("edges", []),
        )

    def incident_graph(
        self,
        incident_id: str,
        depth: int = 3,
    ) -> GraphView:
        """Fetch the graph for a specific incident."""
        resp = self._get(f"/graph/incident/{incident_id}?depth={depth}")
        return GraphView(
            nodes=resp.get("nodes", []),
            edges=resp.get("edges", []),
        )

    def record_remediation(self, remediation: Remediation) -> None:
        """Record a remediation action taken for an incident."""
        self._post("/feedback/remediation", {
            "incident_id": remediation.incident_id,
            "action": remediation.action,
            "operator": remediation.operator,
            "attributes": remediation.attributes,
        })

    def record_feedback(self, feedback: RemediationFeedback) -> None:
        """Record feedback on whether a remediation was successful."""
        self._post("/feedback/incident", {
            "remediation_id": feedback.remediation_id,
            "incident_id": feedback.incident_id,
            "outcome": feedback.outcome,
            "time_to_resolve_seconds": feedback.time_to_resolve_seconds,
            "notes": feedback.notes,
        })

    def similar_incidents(self, incident_id: str, limit: int = 10) -> list[dict]:
        """Find incidents similar to the given one."""
        resp = self._post("/reconstruct", {
            "incident_id": incident_id,
            "mode": "fast",
            "limit": limit,
        })
        results = resp.get("similar_incidents", [])
        return results[:limit] if limit > 0 else results

    def ingest_batch(self, events: list[dict]) -> None:
        """Ingest a batch of events directly into cortex."""
        self._post("/events/batch", {"events": events})

    def ingest_event(self, event: dict) -> None:
        """Ingest a single event directly into cortex."""
        self._post("/events", event)

    def ingest_jsonl(self, events: list[dict]) -> None:
        """Ingest events as a JSONL stream into cortex."""
        url = f"{self.endpoint}/events/jsonl"
        data = "\n".join(json.dumps(e, separators=(",", ":")) for e in events).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=data,
            headers={**self._headers(), "content-type": "application/x-ndjson"},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=self.timeout) as resp:
            resp.read()
