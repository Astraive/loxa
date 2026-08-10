# Event State Machine and Lifecycle Management

## Overview

The LOZA Go SDK implements a strict event lifecycle state machine that ensures predictable and traceable event state transitions. This document describes the state machine implementation, valid transitions, and API methods.

## Requirements

This implementation satisfies the following requirements:
- **1.1**: Support for all event states (created, active, finished, emitting, emitted, validation_failed, delivery_failed)
- **1.2**: Transition to created state on StartEvent
- **1.3**: Transition to active state on Enrich, Set, or Add
- **1.4**: Transition to finished state on Finish or FinishError
- **1.5**: Transition to emitting state on Emit
- **1.6**: Transition to emitted state on successful collector acknowledgment
- **1.7**: Transition to validation_failed state on schema validation failure
- **1.8**: Transition to delivery_failed state on delivery failure after retries
- **1.9**: Rejection of invalid state transitions
- **1.10**: GetState() method for observability
- **1.11**: Enforcement of state transition order: created → active → finished → emitting → emitted
- **2.1**: StartEvent method
- **2.2**: Enrich method
- **2.3**: Set method
- **2.4**: Add method
- **2.5**: Finish method
- **2.6**: FinishError method
- **2.7**: Emit method

## Event States

The SDK defines the following event states:

```go
type EventState string

const (
    EventStateCreated          EventState = "created"
    EventStateActive           EventState = "active"
    EventStateFinished         EventState = "finished"
    EventStateEmitting         EventState = "emitting"
    EventStateEmitted          EventState = "emitted"
    EventStateFailedValidation EventState = "failed_validation"
    EventStateDeliveryFailed   EventState = "delivery_failed"
)
```

### State Descriptions

- **created**: Initial state when an event is created via `StartEvent()`
- **active**: Event is being enriched with data via `Enrich()`, `Set()`, or `Add()`
- **finished**: Event has been marked complete via `Finish()` or `FinishError()`
- **emitting**: Event is being sent to the collector (transient state)
- **emitted**: Event has been successfully delivered to the collector
- **validation_failed**: Event failed schema validation
- **delivery_failed**: Event delivery failed after all retries

## State Transition Diagram

```
                    ┌─────────────┐
                    │   created   │
                    └──────┬──────┘
                           │
                    Enrich/Set/Add
                           │
                           ▼
                    ┌─────────────┐
                    │   active    │
                    └──────┬──────┘
                           │
                   Finish/FinishError
                           │
                           ▼
                    ┌─────────────┐
                    │  finished   │
                    └──────┬──────┘
                           │
                        Emit()
                           │
                           ▼
                    ┌─────────────┐
                    │  emitting   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
       validation    delivery      success
         fails         fails
              │            │            │
              ▼            ▼            ▼
    ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
    │ validation_  │ │ delivery_    │ │   emitted    │
    │   failed     │ │   failed     │ │              │
    └──────────────┘ └──────────────┘ └──────────────┘
```

## Valid State Transitions

| From State          | To State            | Trigger                          |
|---------------------|---------------------|----------------------------------|
| created             | active              | Enrich(), Set(), Add()           |
| active              | finished            | Finish(), FinishError()          |
| finished            | emitting            | Emit()                           |
| emitting            | emitted             | Successful delivery              |
| emitting            | validation_failed   | Schema validation failure        |
| emitting            | delivery_failed     | Delivery failure after retries   |

## Invalid State Transitions

The following transitions are **rejected** with appropriate errors:

- Cannot modify an `emitted` event (returns `EventClosedError`)
- Cannot finish an `emitted` event (returns `EventClosedError`)
- Cannot emit an `emitted` event twice (returns `DuplicateEmitError`)
- Cannot finish an event twice (returns `EventAlreadyFinishedError`)
- Cannot modify a `validation_failed` event (returns `EventClosedError`)
- Cannot modify a `delivery_failed` event (returns `EventClosedError`)

## API Methods

### StartEvent

Creates a new event and returns a context containing the event.

```go
ctx := loza.StartEvent(context.Background(), loza.Params{
    Event: "user.login",
})
```

**State Transition**: None → `created`

### Enrich

Appends attributes to an active event.

```go
err := loza.Enrich(ctx,
    loza.String("user_id", "123"),
    loza.String("email", "user@example.com"),
)
```

**State Transition**: `created` → `active`

### Set

Sets or updates attributes on an active event.

```go
err := loza.Set(ctx,
    loza.String("status", "success"),
    loza.Int("attempts", 3),
)
```

**State Transition**: `created` → `active`

### Add

Appends a value to an array field on an active event.

```go
err := loza.Add(ctx, "tags", "important")
err = loza.Add(ctx, "tags", "urgent")
// Result: tags = ["important", "urgent"]
```

**State Transition**: `created` → `active`

**Behavior**:
- If the field doesn't exist, creates a new array with the value
- If the field exists as an array, appends the value
- If the field exists but is not an array, converts it to an array

### Finish

Marks an event as successfully completed.

```go
err := loza.Finish(ctx, "success",
    loza.Int("items_processed", 42),
)
```

**State Transition**: `active` → `finished`

### FinishError

Marks an event as failed with error details.

```go
err := loza.FinishError(ctx, someError,
    loza.Bool("retryable", true),
)
```

**State Transition**: `active` → `finished`

### Emit

Sends the finished event to the collector.

```go
err := loza.Emit(ctx)
```

**State Transition**: `finished` → `emitting` → `emitted` (or `validation_failed`/`delivery_failed`)

### GetState

Returns the current event state for observability.

```go
ev, _ := loza.FromContext(ctx)
state := ev.State()
// Returns: EventStateCreated, EventStateActive, etc.
```

## Error Types

### EventClosedError

Returned when attempting to modify or finish an event that has reached a terminal state.

```go
type EventClosedError struct {
    EventID string
    State   EventState
}
```

### DuplicateEmitError

Returned when attempting to emit an event that has already been emitted.

```go
type DuplicateEmitError struct {
    EventID string
}
```

### EventAlreadyFinishedError

Returned when attempting to finish an event that has already been finished.

```go
type EventAlreadyFinishedError struct {
    EventID string
}
```

## Thread Safety

All event methods are thread-safe and can be called concurrently. The state machine uses a mutex to ensure atomic state transitions.

## Example Usage

### Complete Event Lifecycle

```go
package main

import (
    "context"
    "errors"
    "github.com/astraive/loza/sdks/go"
)

func main() {
    // Configure the SDK
    cfg := loza.Dev()
    if err := loza.Configure(cfg); err != nil {
        panic(err)
    }
    defer loza.Shutdown(context.Background())

    // Start an event (created state)
    ctx := loza.StartEvent(context.Background(), loza.Params{
        Event: "order.process",
    })

    // Enrich the event (transitions to active)
    loza.Enrich(ctx,
        loza.String("order_id", "ORD-123"),
        loza.String("customer_id", "CUST-456"),
    )

    // Add items to an array
    loza.Add(ctx, "items", "ITEM-1")
    loza.Add(ctx, "items", "ITEM-2")

    // Set additional fields
    loza.Set(ctx,
        loza.Float64("total", 99.99),
        loza.String("currency", "USD"),
    )

    // Finish the event (transitions to finished)
    if err := processOrder(); err != nil {
        loza.FinishError(ctx, err)
    } else {
        loza.Finish(ctx, "success")
    }

    // Emit the event (transitions to emitting → emitted)
    if err := loza.Emit(ctx); err != nil {
        // Handle emission error
    }
}

func processOrder() error {
    // Business logic here
    return nil
}
```

### Error Handling

```go
ctx := loza.StartEvent(context.Background(), loza.Params{
    Event: "test.event",
})

// Finish the event
loza.Finish(ctx, "success")

// Emit the event
loza.Emit(ctx)

// Try to modify after emit - will fail
err := loza.Enrich(ctx, loza.String("key", "value"))
if err != nil {
    var closed *loza.EventClosedError
    if errors.As(err, &closed) {
        // Event is closed, cannot modify
        fmt.Printf("Event %s is in state %s\n", closed.EventID, closed.State)
    }
}

// Try to emit again - will fail
err = loza.Emit(ctx)
if err != nil {
    var duplicate *loza.DuplicateEmitError
    if errors.As(err, &duplicate) {
        // Event already emitted
        fmt.Printf("Event %s already emitted\n", duplicate.EventID)
    }
}
```

## Testing

The state machine implementation includes comprehensive tests:

- `TestEventStateTransitions`: Verifies all valid state transitions
- `TestInvalidStateTransitions`: Verifies rejection of invalid transitions
- `TestGetStateMethod`: Verifies state observability
- `TestAddMethod`: Verifies array field appending
- `TestValidationFailedState`: Verifies validation failure handling
- `TestDeliveryFailedState`: Verifies delivery failure handling
- `TestStateTransitionOrder`: Verifies required transition order

Run tests with:

```bash
go test ./internal/core/ -run StateMachine
```

## Implementation Details

### State Storage

Event state is stored in the `Event` struct:

```go
type Event struct {
    mu      sync.Mutex
    state   EventState
    emitted atomic.Bool
    // ... other fields
}
```

### State Transition Logic

State transitions are validated in several methods:

- `ensureMutableLocked()`: Checks if event can be modified
- `beginEmit()`: Validates and transitions to emitting state
- `markEmitted()`: Transitions to emitted state
- `markValidationFailed()`: Transitions to validation_failed state
- `markDeliveryFailed()`: Transitions to delivery_failed state

### Automatic Transitions

Some methods automatically transition states:

- `AddAttrs()`: Transitions `created` → `active`
- `Enrich()`: Transitions `created` → `active` (via AddAttrs)
- `Set()`: Transitions `created` → `active`
- `Add()`: Transitions `created` → `active`
- `finish()`: Transitions to `finished`
- `beginEmit()`: Transitions to `emitting`

## Best Practices

1. **Always finish before emitting**: Call `Finish()` or `FinishError()` before `Emit()`
2. **Check errors**: Always check errors from state-modifying methods
3. **Use GetState() for debugging**: Query event state when troubleshooting
4. **Handle terminal states**: Be prepared for `EventClosedError` in long-running contexts
5. **Don't retry emitted events**: Check for `DuplicateEmitError` to avoid duplicate emissions

## Future Enhancements

Potential future improvements:

- State transition callbacks for custom logic
- State transition metrics for monitoring
- Configurable state machine behavior
- State persistence for crash recovery
