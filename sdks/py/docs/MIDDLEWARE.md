# Middleware

Python web framework middleware for automatic HTTP event creation. Each middleware captures request metadata and creates a LOXA event per request.

## Available Middleware

| Module | Framework | Description |
|--------|-----------|-------------|
| `loxa.middleware.asgi` | Generic ASGI | Base ASGI middleware for Starlette, FastAPI, and custom apps. |
| `loxa.middleware.django` | Django | Django middleware for request/response event capture. |
| `loxa.middleware.fastapi` | FastAPI | FastAPI dependency/middleware integration. |
| `loxa.middleware.flask` | Flask | Flask WSGI middleware. |
| `loxa.middleware.starlette` | Starlette | Starlette ASGI middleware. |

## ASGI Middleware

The generic ASGI middleware wraps any ASGI application:

```python
from loxa.middleware.asgi import LoxaMiddleware

app = LoxaMiddleware(asgi_app, service="my-service")
```

What it captures:
- HTTP method, path, route
- User-Agent, remote IP
- Response status code
- Request duration (ms)

## FastAPI

```python
from fastapi import FastAPI
from loxa.middleware.fastapi import LoxaMiddleware

app = FastAPI()
app.add_middleware(LoxaMiddleware, service="my-service")

@app.get("/users/{user_id}")
async def get_user(user_id: str):
    return {"user_id": user_id}
```

## Django

Add to `MIDDLEWARE` in `settings.py`:

```python
MIDDLEWARE = [
    "loxa.middleware.django.LoxaMiddleware",
    # ... other middleware
]
```

## Flask

```python
from flask import Flask
from loxa.middleware.flask import LoxaMiddleware

app = Flask(__name__)
LoxaMiddleware(app, service="my-service")
```

## Starlette

```python
from starlette.applications import Starlette
from loxa.middleware.starlette import LoxaMiddleware

app = Starlette()
app.add_middleware(LoxaMiddleware, service="my-service")
```

## Common Behavior

All middleware implementations follow the same pattern:

1. **Request start**: Create an event with `start_http_event`.
2. **Enrich**: Add HTTP metadata (method, path, user-agent, remote IP).
3. **Request end**: Record status code, duration, and outcome.
4. **Emit**: Send the event to the configured sink.

The middleware uses the global default logger. Configure it before adding middleware:

```python
import loxa
loxa.configure(loxa.production("my-service").with_sink(
    loxa.HTTPBatchSink("http://collector:9090/v1/events")
))
```
