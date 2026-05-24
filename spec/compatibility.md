# LOXA Schema Compatibility

This document provides an overview of schema compatibility policies across all versions.

## Current Version

- **Active Schema Version**: v0.0.2
- **Status**: Stable
- **Location**: `v1/`

## Versioning Policy

LOXA schemas follow [Semantic Versioning](https://semver.org/):

- **MAJOR** version (v1 → v2): Breaking changes
- **MINOR** version (v0.0.1): Backward-compatible additions
- **PATCH** version (v0.0.1 → v0.0.1): Documentation updates

## Version-Specific Compatibility

For detailed compatibility information for each version:

- [v1 Compatibility Matrix](v1/compatibility.md)

## Cross-Version Compatibility

### Migration Periods

When a new major version is released:

1. **Announcement**: 30 days before release
2. **Dual Support**: 90 days minimum overlap
3. **Deprecation**: Old version marked deprecated
4. **Removal**: After migration period ends

### Collector Support

Collectors must support:
- Current major version (required)
- Previous major version during migration (required)
- Next major version in beta (optional)

### SDK Support

SDKs should:
- Emit events using current schema version
- Include `schema_version` field in all events
- Support graceful degradation for unknown fields

## Compatibility Testing

### Validation Tools

```bash
# Validate against specific version
loxa schema validate --file event.json --schema-version v1

# Check compatibility between versions
loxa schema diff --from v0.0.1 --to v0.0.1

# List all available versions
loxa schema list
```

### Golden Test Suites

Each version includes golden test files:
- `v1/examples/` - Valid example events
- `examples/golden/valid/` - Events that must pass validation
- `examples/golden/invalid/` - Events that must fail validation

## Breaking vs Non-Breaking Changes

### ✅ Non-Breaking (Minor/Patch)

- Adding optional fields
- Adding new enum values
- Relaxing constraints (e.g., removing minLength)
- Documentation updates
- Example additions

### ❌ Breaking (Major)

- Removing fields
- Changing field types
- Adding required fields
- Removing enum values
- Tightening constraints

## Schema Evolution Process

1. **Proposal**: Submit GitHub issue with proposed changes
2. **Review**: Community review and feedback (7 days minimum)
3. **Implementation**: Create PR with schema changes
4. **Testing**: Validate against golden test suite
5. **Documentation**: Update CHANGELOG and compatibility docs
6. **Release**: Tag and publish new version

## Backward Compatibility Guarantee

Within a major version (e.g., all v1.x.x releases):

- **Producers** (SDKs): Can emit events using any v1.x.x schema
- **Consumers** (Collectors): Must accept events from any v1.x.x schema
- **Unknown Fields**: Must be preserved or ignored (not rejected)

## Forward Compatibility

Older collectors should handle newer minor versions gracefully:

- Accept events with unknown optional fields
- Ignore unrecognized fields (don't reject)
- Log warnings for unexpected fields (optional)

## Version Detection

Events should include version information:

```json
{
  "schema_version": "v1",
  "event_id": "...",
  "event_type": "...",
  ...
}
```

Collectors use this to:
- Select appropriate validator
- Apply version-specific processing
- Track version distribution metrics

## Deprecation Policy

When deprecating features:

1. Mark as deprecated in schema using `x-loxa-deprecated`
2. Document migration path
3. Maintain support for 90 days minimum
4. Log deprecation warnings
5. Remove in next major version

Example:

```json
{
  "old_field": {
    "type": "string",
    "x-loxa-deprecated": "use new_field instead",
    "x-loxa-deprecated-since": "v0.0.1",
    "x-loxa-removed-in": "v0.0.1"
  }
}
```

## Support Status

| Version | Status | Released | Support Ends | Notes |
|---------|--------|----------|--------------|-------|
| v0.0.2  | Active | 2026-05-21 | N/A | Current stable |

## Resources

- [Schema Evolution Policy](docs/SCHEMA_EVOLUTION_POLICY.md)
- [Release Compatibility](docs/RELEASE_COMPATIBILITY.md)
- [Schema Versioning](docs/SCHEMA_VERSIONING.md)

## Contact

For compatibility questions:
- GitHub Issues: https://github.com/astraive/loxa/issues
- Documentation: https://github.com/astraive/loxa/tree/main/spec/docs
