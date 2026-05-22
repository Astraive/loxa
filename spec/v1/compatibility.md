# Schema Compatibility Matrix - v1

This document defines the compatibility rules and version coexistence policies for LOXA event schema v1.

## Version Information

- **Schema Version**: v1.0.0
- **Status**: Active
- **Release Date**: 2024-01-15
- **Compatibility Level**: Baseline

## Semantic Versioning

LOXA schemas follow semantic versioning (MAJOR.MINOR.PATCH):

- **MAJOR** (v1 → v2): Breaking changes
  - Field removal
  - Field type changes
  - Required field additions
  - Enum value removal
  
- **MINOR** (v1.0.0): Backward-compatible additions
  - New optional fields
  - New enum values
  - Documentation enhancements
  
- **PATCH** (v1.0.0 → v1.0.0): Non-functional changes
  - Documentation fixes
  - Example updates
  - Clarifications

## Compatibility Guarantees

### Within Major Version (v1.0.0)

- **Backward Compatible**: All v1.0.0 schemas are backward compatible
- **Forward Compatible**: Older collectors can process events from newer v1.0.0 schemas (unknown fields ignored)
- **Migration Period**: Not required for minor/patch updates

### Across Major Versions (v1 → v2)

- **Breaking Changes**: May introduce incompatibilities
- **Migration Period**: 90 days minimum overlap support
- **Dual Support**: Collectors must support both versions during migration
- **Deprecation Notice**: 30 days minimum before removal

## Version Coexistence

### Supported Combinations

| SDK Version | Collector Version | Status |
|-------------|-------------------|--------|
| v1.0.0      | v1.0.0           | ✅ Fully Supported |
| v1.0.0      | v1.0.0           | ✅ Supported (migration) |
| v1.0.0      | v1.0.0           | ⚠️ Limited (v2 features unavailable) |

### Migration Strategy

1. **Phase 1**: Deploy v2 collectors (support both v1 and v2)
2. **Phase 2**: Upgrade SDKs to v2 (emit v2 events)
3. **Phase 3**: Deprecate v1 support after migration period
4. **Phase 4**: Remove v1 support from collectors

## Field Evolution Rules

### Adding Fields

✅ **Allowed** (Minor version bump):
- New optional fields
- New nested objects
- New enum values

❌ **Not Allowed** (Major version required):
- New required fields
- Changing field types
- Removing enum values

### Modifying Fields

✅ **Allowed** (Patch version):
- Documentation updates
- Example additions
- Description clarifications

❌ **Not Allowed** (Major version required):
- Type changes (string → number)
- Format changes (date → date-time)
- Constraint tightening (minLength increase)

### Removing Fields

❌ **Never Allowed** in minor/patch versions

✅ **Allowed** in major version with:
- Deprecation notice (30 days minimum)
- Migration guide
- Alternative field recommendation

## Validation Modes

Collectors support multiple validation modes for compatibility:

1. **Strict**: Reject events that don't match schema exactly
2. **Lenient**: Accept events with unknown fields (forward compatibility)
3. **Warn**: Log warnings for schema violations but accept events
4. **Off**: No validation (not recommended for production)

## Schema Extensions

### Custom Fields

- Use `data` object for event-specific fields
- Use `additionalProperties: true` for extensibility
- Prefix custom fields with `x-` for vendor extensions

### PII Annotations

- Use `x-pii` for privacy classification
- Values: `internal`, `confidential`, `restricted`
- Collectors must respect PII annotations for redaction

## Testing Compatibility

### Validation Commands

```bash
# Validate event against v1 schema
loxa schema validate --file event.json --schema-version v1

# Check compatibility between versions
loxa schema diff --from v1.0.0 --to v1.0.0

# List all schema versions
loxa schema list
```

### Golden Test Files

Use golden test files in `examples/golden/` for validation:

- `valid/` - Events that must pass validation
- `invalid/` - Events that must fail validation

## Breaking Change Examples

### ❌ Breaking: Removing a field

```json
// v1.0.0
{
  "event_id": "...",
  "event_type": "...",
  "deprecated_field": "value"  // ← Removed in v1.0.0
}
```

### ❌ Breaking: Changing field type

```json
// v1.0.0
{
  "duration_ms": 123  // number
}

// v1.0.0 (BREAKING)
{
  "duration_ms": "123ms"  // string
}
```

### ✅ Non-Breaking: Adding optional field

```json
// v1.0.0
{
  "event_id": "...",
  "event_type": "..."
}

// v1.0.0 (Compatible)
{
  "event_id": "...",
  "event_type": "...",
  "new_optional_field": "value"  // ← New optional field
}
```

## Version Detection

Events should include schema version for proper validation:

```json
{
  "schema_version": "v1",
  "event_id": "...",
  ...
}
```

Collectors use `schema_version` field to:
- Select appropriate validator
- Apply version-specific processing
- Track version distribution

## Deprecation Policy

When deprecating fields:

1. Mark field as deprecated in schema with `x-loxa-deprecated`
2. Provide migration path in documentation
3. Maintain support for 90 days minimum
4. Log deprecation warnings in collector
5. Remove in next major version

Example:

```json
{
  "environment": {
    "type": "string",
    "x-loxa-deprecated": "use deployment.environment",
    "x-loxa-since": "v1"
  }
}
```

## Support Matrix

| Version | Status | Support End Date | Notes |
|---------|--------|------------------|-------|
| v1.0.0  | Active | N/A              | Current stable version |

## Contact

For compatibility questions or concerns:
- GitHub Issues: https://github.com/Astraive/loxa/issues
- Documentation: https://github.com/Astraive/loxa/docs
