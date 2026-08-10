# Schema Evolution Policy

LOZA schema versions follow compatibility-first evolution.

Version rules:

- Patch: bug fixes only. No new required fields and no behavior-breaking type changes.
- Minor: optional fields, optional enum values, and new metadata annotations are allowed.
- Major: breaking field, type, semantic, or required-field changes.

Compatibility rules:

- Deprecated fields remain accepted for at least `N` minor versions. The default is `N=2` unless a release note says otherwise.
- The collector can accept older schema versions through compatibility adapters.
- SDKs declare the supported schema range they can emit and parse.
- Strict mode rejects unknown required fields and unsupported major versions.
- Loose mode preserves unknown fields and validates only the known canonical contract.

Spec metadata annotations:

- `x-loza-since`: first schema version where the field is valid.
- `x-loza-deprecated`: human-readable deprecation reason.
- `x-loza-until`: last schema version where the field is accepted.

