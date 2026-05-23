import { Logger } from '../core/logger.ts';
import { String as AttrString, Int as AttrInt } from '../core/event.ts';

export interface HttpMiddlewareOptions {
  service?: string;
}

export function loxaHttpMiddleware(opts: HttpMiddlewareOptions = {}) {
  const loxa = new Logger({ service: opts.service });

  return (req: any, res: any, next: any) => {
    const startedAt = Date.now();

    const ev = loxa.startHTTPEvent({
      event: `${req.method} ${req.url}`,
      kind: 'http',
      method: req.method,
      path: req.url,
      service: opts.service,
    });

    loxa.enrich(ev,
      AttrString('http.user_agent', req.headers?.['user-agent'] || ''),
      AttrString('http.remote_ip', req.socket?.remoteAddress || ''),
    );

    const originalEnd = res.end;
    res.end = function (...args: any[]) {
      const durationMs = Date.now() - startedAt;
      const outcome = res.statusCode >= 500 ? 'error' : 'success';
      loxa.finish(ev, outcome,
        AttrInt('status_code', res.statusCode),
        AttrInt('duration_ms', durationMs),
      );
      loxa.emit(ev).catch(() => {});
      return originalEnd.apply(res, args);
    };

    res.on('error', (err: Error) => {
      loxa.finishError(ev, err);
      loxa.emit(ev).catch(() => {});
    });

    next();
  };
}
