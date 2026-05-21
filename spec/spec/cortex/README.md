# LOXA Cortex Specification

This directory contains the formal specification for LOXA Cortex, the incident detection and reconstruction engine that consumes canonical LOXA events.

## Overview

Cortex is a deterministic, graph-based system that:

- Ingests canonical Loxa events, OTel signals, and raw logs
- Normalizes multi-source data into a unified event model
- Builds service dependency graphs and incident graphs
- Reconstructs incident timelines with RCA (root cause analysis)
- Learns from remediation feedback

## Specification Files

- **CORTEX_ARCHITECTURE.md** - System design and component model
- **CORTEX_EVENT_MODEL.md** - Canonical event ingest contract
- **CORTEX_INGEST.md** - Ingest API and batch operations
- **CORTEX_GRAPH.md** - Graph node/edge model and APIs
- **CORTEX_RECONSTRUCTION.md** - Timeline and RCA reconstruction
- **CORTEX_FEEDBACK.md** - Remediation feedback and learning
- **CORTEX_STORAGE.md** - Storage interfaces and persistence
- **CORTEX_SECURITY.md** - Authentication, authorization, PII redaction
- **CORTEX_CONFORMANCE.md** - Test fixtures and validation rules

## Event Flow

```
Loxa Collector → Cortex Ingest → Normalization → Processor
                                                      ↓
                                              Graph Building
                                                      ↓
                                           Incident Detection
                                                      ↓
                                            Reconstruction
                                                      ↓
                                         Feedback & Learning
```

## Core Contracts

- **Canonical Event**: `schemas/json/cortex-event.schema.json`
- **Graph Node**: `schemas/json/cortex-graph-node.schema.json`
- **Graph Edge**: `schemas/json/cortex-graph-edge.schema.json`
- **Reconstruction Response**: `schemas/json/cortex-reconstruct-response.schema.json`
- **Feedback**: `schemas/json/cortex-feedback.schema.json`

## Key Principles

1. **Contract-First**: All data flows through schemas first
2. **Deterministic**: Same inputs always produce same results
3. **Testable**: All behavior has conformance fixtures
4. **Extensible**: New graph types and edge types can be added
5. **Observable**: Full audit trail via feedback loop
