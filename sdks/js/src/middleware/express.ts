import { Logger } from '../core/logger.ts';
import { String as AttrString, Int as AttrInt } from '../core/event.ts';

export interface ExpressMiddlewareOptions {
  service?: string;
  routeExtractor?: (req: any) => string;
}

export function lozaMiddleware(opts: ExpressMiddlewareOptions = {}) {
  const loza = new Logger({ service: opts.service });

  return (req: any, res: any, next: any) => {
    const startedAt = Date.now();
    const route = opts.routeExtractor?.(req) || req.route?.path || req.path || '';

    const ctx = loza.startHTTPEvent({
      event: `${req.method} ${route}`,
      kind: 'http',
      method: req.method,
      path: req.path,
      route,
      service: opts.service,
    });

    loza.enrich(ctx,
      AttrString('http.user_agent', req.headers?.['user-agent'] || ''),
      AttrString('http.remote_ip', req.ip || req.socket?.remoteAddress || ''),
    );

    const originalEnd = res.end;
    res.end = function (...args: any[]) {
      const durationMs = Date.now() - startedAt;
      const outcome = res.statusCode >= 500 ? 'error' : 'success';
      loza.finish(ctx, outcome,
        AttrInt('status_code', res.statusCode),
        AttrInt('duration_ms', durationMs),
      );
      loza.emit(ctx).catch(() => {});
      return originalEnd.apply(res, args);
    };

    res.on('error', (err: Error) => {
      loza.finishError(ctx, err);
      loza.emit(ctx).catch(() => {});
    });

    next();
  };
}
