# Config-As-Code Registry

Collector config uses OTEL-like component registries:

- receivers registry
- processors registry
- exporters registry
- extensions registry

Config validation MUST catch:

- unknown component
- unused component
- pipeline references missing component
- cycle in routing
- duplicate sink names
- secret accidentally printed

