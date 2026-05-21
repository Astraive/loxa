# Event State Machine Example

This example demonstrates the LOXA event state machine and lifecycle management.

## Overview

The LOXA SDK implements a strict event lifecycle state machine with the following states:

- **created**: Initial state when event is created
- **active**: Event is being enriched with data
- **finished**: Event has been marked complete
- **emitting**: Event is being sent to collector (transient)
- **emitted**: Event successfully delivered
- **validation_failed**: Event failed schema validation
- **delivery_failed**: Event delivery failed after retries

## Running the Example

```bash
cd examples/state-machine
go run main.go
```

## What This Example Demonstrates

### 1. Complete Event Lifecycle

Shows the happy path through all states:
- Create event (created)
- Enrich with data (active)
- Set fields (active)
- Finish event (finished)
- Emit event (emitted)

### 2. Error Handling

Demonstrates error event lifecycle:
- Create event
- Enrich with data
- Finish with error
- Emit error event

### 3. Invalid State Transitions

Shows what happens when you try invalid operations:
- Modifying an emitted event (fails with EventClosedError)
- Finishing an emitted event (fails with EventClosedError)
- Emitting an event twice (fails with DuplicateEmitError)

### 4. Array Field Operations

Demonstrates the Add method for array fields:
- Adding items to an array
- Creating arrays on-the-fly
- Multiple array fields

## Key Concepts

### State Transitions

Valid transitions follow this order:
```
created → active → finished → emitting → emitted
```

### Methods and State Changes

- `StartEvent()`: Creates event in `created` state
- `Enrich()`, `Set()`, `Add()`: Transition to `active` state
- `Finish()`, `FinishError()`: Transition to `finished` state
- `Emit()`: Transitions through `emitting` to `emitted` state

### Error Types

- `EventClosedError`: Returned when trying to modify a terminal state event
- `DuplicateEmitError`: Returned when trying to emit an already-emitted event
- `EventAlreadyFinishedError`: Returned when trying to finish an already-finished event

## Expected Output

```
=== LOXA Event State Machine Demo ===

Example 1: Complete Event Lifecycle
1. Initial state: created
2. After Enrich: active
3. After Set: active
4. After Finish: finished
5. After Emit: emitted

Example 2: Error Handling
1. Initial state: created
2. After Enrich: active
3. After FinishError: finished
4. After Emit: emitted

Example 3: Invalid State Transitions
Event state: emitted

Attempting to modify emitted event:
✗ Cannot modify: Event <id> is in state emitted

Attempting to finish emitted event:
✗ Cannot finish: Event <id> is in state emitted

Attempting to emit again:
✗ Cannot emit: Event <id> already emitted

Example 4: Array Field Operations
Initial state: created

Adding items to cart:
  Added ITEM-001
  Added ITEM-002
  Added ITEM-003

Cart items: [ITEM-001 ITEM-002 ITEM-003]

Adding tags:
Tags: [priority express-shipping]

State after Add operations: active
Final state: emitted
```

## Related Documentation

- [State Machine Documentation](../../docs/state-machine.md)
- [Event Lifecycle Documentation](../../docs/event-lifecycle.md)
- [Public API Documentation](../../docs/public-api.md)
