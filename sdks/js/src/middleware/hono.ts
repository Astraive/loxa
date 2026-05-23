import { Logger } from '../core/logger.ts';
import { String as AttrString, Int as AttrInt } from '../core/event.ts';

export interface HonoMiddlewareOptions {
  service?: string;
}

export function loxaHonoMiddleware(opts: HonoMiddlewareOptions = {}) {
  const loxa = new Logger({ service: opts.service });

  return async (c: any, next: any) => {
    const startedAt = Date.now();
    const route = c.req.routePath || c.req.path || '';

    const ev = loxa.startHTTPEvent({
      event: `${c.req.method} ${route}`,
      kind: 'http',
      method: c.req.method,
      path: c.req.path,
      route,
      service: opts.service,
    });

    loxa.enrich(ev,
      AttrString('http.user_agent', c.req.header('user-agent') || ''),
      AttrString('http.remote_ip', c.req.header('x-forwarded-for') || ''),
    );

    try {
      await next();
      const durationMs = Date.now() - startedAt;
      const outcome = c.res.status >= 500 ? 'error' : 'success';
      loxa.finish(ev, outcome,
        AttrInt('status_code', c.res.status),
        AttrInt('duration_ms', durationMs),
      );
    } catch (err: any) {
      const durationMs = Date.now() - startedAt;
      loxa.finishError(ev, err,
        AttrInt('duration_ms', durationMs),
      );
    }
    await loxa.emit(ev).catch(() => {});
  };
}
