# Loza Business Instrumentation

Loza v0.2.0 treats business instrumentation as a lifecycle, not a log line. Applications describe the full story of a checkout, payment, login, job, webhook, AI agent run, or RAG pipeline, while OpenTelemetry continues to describe distributed trace movement.

The canonical lifecycle is:

```text
startEvent -> append/enrich -> checkpoint -> process/group/timer -> finish/finishError -> emit
```

## Default Client

Use the default client for simple applications and examples:

```ts
const event = loza.startEvent({ event: "checkout.request", kind: "http" });
event.append(loza.userId(user.id), loza.int("cart.item_count", items.length));
event.checkpoint("cart_loaded");
event.finish("success");
await event.emit();
```

## Custom Client / Alias

Use `createLoza` / `create_loza` / `CreateLoza` for an independent client with its own config. Use `alias(name)` when you want the same config with logical alias metadata.

```ts
const payments = createLoza({ service: "checkout-api" });
const audit = loza.alias("audit");
```

`alias(name)` preserves the parent config and adds `loza.alias` to emitted events. It does not change `service` and does not mutate the parent client.

## Primitive Choice

| Primitive | Use For |
| --- | --- |
| `checkpoint` | Breadcrumbs such as `cart_loaded` or `prompt_built` |
| `process` | Ordered business steps such as `payment.authorize` |
| `group` | Larger phases such as `payment_flow` or `rag_retrieval_phase` |
| `timer` | Independent latency measurements such as `db.cart_lookup` |
| `stopwatch` | Local elapsed time before attaching it elsewhere |
| `link` | Correlation to another event, trace, span, job, or external ID |

## Cross-Language Parity

| JavaScript | Python | Go | Rust |
| --- | --- | --- | --- |
| `createLoza()` | `create_loza()` | `CreateLoza()` | `create_loza()` |
| `loza.info()` | `loza.info()` | `loza.Info(ctx)` | `loza::info()` |
| `logger.info()` | `logger.info()` | `logger.Info(ctx)` | `logger.info()` |
| `event.process()` | `event.process()` | `event.Process()` | `event.process()` |

For the full v0.2.0 product and SDK method catalog, see [instrumentation-and-sdk-idea.md](./instrumentation-and-sdk-idea.md).
