package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/astraive/loxa-go"
)

func main() {
	// Configure the SDK
	cfg := loxa.Dev()
	if err := loxa.Configure(cfg); err != nil {
		panic(err)
	}
	defer loxa.Shutdown(context.Background())

	fmt.Println("=== LOXA Event State Machine Demo ===")

	// Example 1: Happy path - complete lifecycle
	fmt.Println("Example 1: Complete Event Lifecycle")
	happyPath()
	fmt.Println()

	// Example 2: Error handling
	fmt.Println("Example 2: Error Handling")
	errorHandling()
	fmt.Println()

	// Example 3: Invalid state transitions
	fmt.Println("Example 3: Invalid State Transitions")
	invalidTransitions()
	fmt.Println()

	// Example 4: Array field operations with Add
	fmt.Println("Example 4: Array Field Operations")
	arrayOperations()
	fmt.Println()
}

func happyPath() {
	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event: "order.process",
	})

	ev, _ := loxa.FromContext(ctx)
	fmt.Printf("1. Initial state: %s\n", ev.State())

	// Enrich transitions to active
	loxa.Enrich(ctx,
		loxa.String("order_id", "ORD-123"),
		loxa.String("customer_id", "CUST-456"),
	)
	fmt.Printf("2. After Enrich: %s\n", ev.State())

	// Set also keeps it active
	loxa.Set(ctx,
		loxa.Float64("total", 99.99),
		loxa.String("currency", "USD"),
	)
	fmt.Printf("3. After Set: %s\n", ev.State())

	// Finish transitions to finished
	loxa.Finish(ctx, "success",
		loxa.Int("items_processed", 3),
	)
	fmt.Printf("4. After Finish: %s\n", ev.State())

	// Emit transitions to emitted
	if err := loxa.Emit(ctx); err != nil {
		fmt.Printf("   Emit error: %v\n", err)
	}
	fmt.Printf("5. After Emit: %s\n", ev.State())
}

func errorHandling() {
	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event: "payment.process",
	})

	ev, _ := loxa.FromContext(ctx)
	fmt.Printf("1. Initial state: %s\n", ev.State())

	loxa.Enrich(ctx,
		loxa.String("payment_id", "PAY-789"),
		loxa.Float64("amount", 49.99),
	)
	fmt.Printf("2. After Enrich: %s\n", ev.State())

	// Simulate an error
	paymentErr := errors.New("insufficient funds")
	loxa.FinishError(ctx, paymentErr,
		loxa.Bool("retryable", true),
		loxa.String("error_code", "INSUFFICIENT_FUNDS"),
	)
	fmt.Printf("3. After FinishError: %s\n", ev.State())

	// Emit the error event
	if err := loxa.Emit(ctx); err != nil {
		fmt.Printf("   Emit error: %v\n", err)
	}
	fmt.Printf("4. After Emit: %s\n", ev.State())
}

func invalidTransitions() {
	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event: "test.invalid",
	})

	ev, _ := loxa.FromContext(ctx)

	// Complete the lifecycle
	loxa.Enrich(ctx, loxa.String("key", "value"))
	loxa.Finish(ctx, "success")
	loxa.Emit(ctx)

	fmt.Printf("Event state: %s\n", ev.State())

	// Try to modify after emit
	fmt.Println("\nAttempting to modify emitted event:")
	if err := loxa.Enrich(ctx, loxa.String("new_key", "new_value")); err != nil {
		var closed *loxa.EventClosedError
		if errors.As(err, &closed) {
			fmt.Printf("✗ Cannot modify: Event %s is in state %s\n", closed.EventID, closed.State)
		}
	}

	// Try to finish after emit
	fmt.Println("\nAttempting to finish emitted event:")
	if err := loxa.Finish(ctx, "success"); err != nil {
		var closed *loxa.EventClosedError
		if errors.As(err, &closed) {
			fmt.Printf("✗ Cannot finish: Event %s is in state %s\n", closed.EventID, closed.State)
		}
	}

	// Try to emit again
	fmt.Println("\nAttempting to emit again:")
	if err := loxa.Emit(ctx); err != nil {
		var duplicate *loxa.DuplicateEmitError
		if errors.As(err, &duplicate) {
			fmt.Printf("✗ Cannot emit: Event %s already emitted\n", duplicate.EventID)
		}
	}
}

func arrayOperations() {
	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event: "shopping.cart",
	})

	ev, _ := loxa.FromContext(ctx)
	fmt.Printf("Initial state: %s\n", ev.State())

	// Add items to cart using Add method
	fmt.Println("\nAdding items to cart:")
	loxa.Add(ctx, "items", "ITEM-001")
	fmt.Println("  Added ITEM-001")

	loxa.Add(ctx, "items", "ITEM-002")
	fmt.Println("  Added ITEM-002")

	loxa.Add(ctx, "items", "ITEM-003")
	fmt.Println("  Added ITEM-003")

	// Retrieve the items
	if items, ok := ev.Get("items"); ok {
		fmt.Printf("\nCart items: %v\n", items)
	}

	// Add tags
	fmt.Println("\nAdding tags:")
	loxa.Add(ctx, "tags", "priority")
	loxa.Add(ctx, "tags", "express-shipping")

	if tags, ok := ev.Get("tags"); ok {
		fmt.Printf("Tags: %v\n", tags)
	}

	fmt.Printf("\nState after Add operations: %s\n", ev.State())

	// Complete the event
	loxa.Finish(ctx, "success")
	loxa.Emit(ctx)
	fmt.Printf("Final state: %s\n", ev.State())
}

func demonstrateStateObservability() {
	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event: "state.tracking",
	})

	ev, _ := loxa.FromContext(ctx)

	states := []string{}
	states = append(states, string(ev.State()))

	loxa.Enrich(ctx, loxa.String("step", "1"))
	states = append(states, string(ev.State()))

	loxa.Set(ctx, loxa.String("step", "2"))
	states = append(states, string(ev.State()))

	loxa.Finish(ctx, "success")
	states = append(states, string(ev.State()))

	loxa.Emit(ctx)
	states = append(states, string(ev.State()))

	fmt.Println("State progression:")
	for i, state := range states {
		fmt.Printf("  %d. %s\n", i+1, state)
	}
}
