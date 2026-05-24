"""Persistent Context Engine — Python adapter for the cortex server."""

from __future__ import annotations

import json
import urllib.request
from typing import Iterable
from urllib.error import HTTPError, URLError

from .context import CausalEdge, Context, IncidentMatch
from .models import Remediation


class Engine:
    """Python adapter for the LOXA Cortex Persistent Context Engine.

    Wraps the cortex HTTP API to provide the benchmark-required interface:
    ``ingest(events)`` and ``reconstruct_context(signal, mode)``.

    Example::

        from loxa.cortex import Engine

        engine = Engine("http://localhost:8080")
        engine.ingest(events)
        ctx = engine.reconstruct_context({"incident_id": "INC-714"}, mode="fast")
    """

    def __init__(self, cortex_url: str = "http://localhost:8080") -> None:
        self.cortex_url = cortex_url.rstrip("/")

    def ingest(self, events: Iterable[dict]) -> None:
        """Ingest a stream of events into the engine.

        Sends events in batches to ``POST /events/batch``.
        """
        batch = []
        for event in events:
            batch.append(event)
            if len(batch) >= 256:
                self._send_batch(batch)
                batch = []
        if batch:
            self._send_batch(batch)

    def reconstruct_context(
        self,
        signal: dict,
        mode: str = "fast",
    ) -> Context:
        """Reconstruct incident context from a signal.

        Calls ``POST /reconstruct`` and maps the cortex response
        to the benchmark-required ``Context`` shape.
        """
        body = {
            "incident_id": signal.get("incident_id", signal.get("signal_id", "")),
            "mode": mode,
        }
        resp = self._post("/reconstruct", body)
        return self._map_response(resp)

    def _send_batch(self, events: list[dict]) -> None:
        self._post("/events/batch", {"events": events})

    def _post(self, path: str, body: dict) -> dict:
        url = f"{self.cortex_url}{path}"
        data = json.dumps(body).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=data,
            headers={"content-type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=30.0) as resp:
                raw = resp.read()
                if raw:
                    return json.loads(raw)
                return {}
        except HTTPError as exc:
            raw = exc.read()
            raise RuntimeError(f"cortex API error {exc.code}: {raw}") from exc
        except URLError as exc:
            raise RuntimeError(f"cortex connection error: {exc}") from exc

    def _get(self, path: str) -> dict:
        url = f"{self.cortex_url}{path}"
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=10.0) as resp:
            return json.loads(resp.read())

    def _map_response(self, resp: dict) -> Context:
        """Map cortex IncidentContext response to benchmark Context shape."""
        # Related events
        related_events = []
        for ce in resp.get("causal_chain", []):
            related_events.append({
                "event_id": ce.get("event_id", ""),
                "ts": ce.get("timestamp", ""),
                "kind": ce.get("kind", ""),
                "service": ce.get("service", ""),
            })

        # Causal chain
        causal_chain = []
        for ce in resp.get("causal_chain", []):
            causal_chain.append(CausalEdge(
                cause_id=ce.get("attributes", {}).get("cause_id", ""),
                effect_id=ce.get("event_id", ""),
                evidence=ce.get("description", ""),
                confidence=ce.get("attributes", {}).get("confidence", 0.5),
            ))

        # Similar incidents
        similar = []
        for si in resp.get("similar_incidents", []):
            similar.append(IncidentMatch(
                past_incident_id=si.get("incident_id", ""),
                similarity=si.get("similarity", 0.0),
                rationale=si.get("shape", ""),
            ))

        # Remediation
        remediations = []
        for ra in resp.get("suggested_actions", []):
            remediations.append(Remediation(
                incident_id=resp.get("incident_id", ""),
                action=ra.get("action", ""),
                attributes={
                    "success_rate": ra.get("success_rate", 0.0),
                    "avg_time_to_resolve": ra.get("avg_time_to_resolve_seconds", 0),
                    "priority": ra.get("priority", 0),
                },
            ))

        return Context(
            related_events=related_events,
            causal_chain=causal_chain,
            similar_past_incidents=similar,
            suggested_remediations=remediations,
            confidence=resp.get("confidence", 0.0),
            explain=resp.get("explain", ""),
        )
