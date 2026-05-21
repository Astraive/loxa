"""Cortex API response models."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class IncidentContext:
    """Result of incident reconstruction."""

    incident_id: str = ""
    timestamp: str = ""
    causal_chain: list[dict] = field(default_factory=list)
    related_services: list[str] = field(default_factory=list)
    symptoms: list[dict] = field(default_factory=list)
    suggested_actions: list[dict] = field(default_factory=list)
    confidence: float = 0.0
    similar_incidents: list[dict] | None = None
    explain: str = ""
    explain_report: dict | None = None


@dataclass
class GraphView:
    """Service or incident dependency graph."""

    nodes: list[dict] = field(default_factory=list)
    edges: list[dict] = field(default_factory=list)


@dataclass
class Remediation:
    """A remediation action taken for an incident."""

    remediation_id: str = ""
    incident_id: str = ""
    action: str = ""
    timestamp: str = ""
    operator: str = ""
    attributes: dict = field(default_factory=dict)


@dataclass
class RemediationFeedback:
    """Feedback on whether a remediation worked."""

    feedback_id: str = ""
    remediation_id: str = ""
    incident_id: str = ""
    outcome: str = ""
    time_to_resolve_seconds: int = 0
    timestamp: str = ""
    notes: str = ""
