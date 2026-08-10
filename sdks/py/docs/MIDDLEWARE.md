# Middleware

Python web framework middleware for automatic HTTP event creation. Each middleware captures request metadata and creates a LOZA event per request.

## Available Middleware

| Module | Framework | Description |
|--------|-----------|-------------|
| `loza.middleware.asgi` | Generic ASGI | Base ASGI middleware for Starlette, FastAPI, and custom apps. |
| `loza.middleware.django` | Django | Django middleware for request/response event capture. |
| `loza.middleware.fastapi` | FastAPI | FastAPI dependency/middleware integration. |
| `loza.middleware.flask` | Flask | Flask WSGI middleware. |
| `loza.middleware.starlette` | Starlette | Starlette ASGI middleware. |

## ASGI Middleware

The generic ASGI middleware wraps any ASGI application:

```python
from loza.middleware.asgi import LozaMiddleware

app = LozaMiddleware(asgi_app, service="my-service")
```

What it captures:
- HTTP method, path, route
- User-Agent, remote IP
- Response status code
- Request duration (ms)

## FastAPI

```python
from fastapi import FastAPI
from loza.middleware.fastapi import LozaMiddleware

app = FastAPI()
app.add_middleware(LozaMiddleware, service="my-service")

@app.get("/users/{user_id}")
async def get_user(user_id: str):
    return {"user_id": user_id}
```

## Django

Add to `MIDDLEWARE` in `settings.py`:

```python
MIDDLEWARE = [
    "loza.middleware.django.LozaMiddleware",
    # ... other middleware
]
```

## Flask

```python
from flask import Flask
from loza.middleware.flask import LozaMiddleware

app = Flask(__name__)
LozaMiddleware(app, service="my-service")
```

## Starlette

```python
from starlette.applications import Starlette
from loza.middleware.starlette import LozaMiddleware

app = Starlette()
app.add_middleware(LozaMiddleware, service="my-service")
```

## Common Behavior

All middleware implementations follow the same pattern:

1. **Request start**: Create an event with `start_http_event`.
2. **Enrich**: Add HTTP metadata (method, path, user-agent, remote IP).
3. **Request end**: Record status code, duration, and outcome.
4. **Emit**: Send the event to the configured sink.

The middleware uses the global default logger. Configure it before adding middleware:

```python
import loza
loza.configure(loza.production("my-service").with_sink(
    loza.HTTPBatchSink("http://collector:9308/events")
))
```
