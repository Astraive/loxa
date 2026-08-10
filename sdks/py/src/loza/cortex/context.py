"""Context output types for the Persistent Context Engine benchmark."""

from __future__ import annotations

from dataclasses import dataclass, field

from .models import Remediation


@dataclass
class CausalEdge:
    """A causal relationship between two events."""

    cause_id: str = ""
    effect_id: str = ""
    evidence: str = ""
    confidence: float = 0.0


@dataclass
class IncidentMatch:
    """A historically similar incident."""

    past_incident_id: str = ""
    similarity: float = 0.0
    rationale: str = ""


@dataclass
class Context:
    """Structured output of incident context reconstruction.

    This is the benchmark-required output shape. Not free text --
    the explain field contains a human-readable narrative.
    """

    related_events: list[dict] = field(default_factory=list)
    causal_chain: list[CausalEdge] = field(default_factory=list)
    similar_past_incidents: list[IncidentMatch] = field(default_factory=list)
    suggested_remediations: list[Remediation] = field(default_factory=list)
    confidence: float = 0.0
    explain: str = ""
