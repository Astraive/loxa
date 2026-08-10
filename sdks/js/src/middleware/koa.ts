import { Logger } from '../core/logger.ts';
import { String as AttrString, Int as AttrInt } from '../core/event.ts';

export interface KoaMiddlewareOptions {
  service?: string;
}

export function lozaKoaMiddleware(opts: KoaMiddlewareOptions = {}) {
  const loza = new Logger({ service: opts.service });

  return async (ctx: any, next: any) => {
    const startedAt = Date.now();
    const route = ctx.route?.path || ctx.path || '';

    const ev = loza.startHTTPEvent({
      event: `${ctx.method} ${route}`,
      kind: 'http',
      method: ctx.method,
      path: ctx.path,
      route,
      service: opts.service,
    });

    loza.enrich(ev,
      AttrString('http.user_agent', ctx.headers?.['user-agent'] || ''),
      AttrString('http.remote_ip', ctx.ip || ctx.socket?.remoteAddress || ''),
    );

    try {
      await next();
      const durationMs = Date.now() - startedAt;
      const outcome = ctx.status >= 500 ? 'error' : 'success';
      loza.finish(ev, outcome,
        AttrInt('status_code', ctx.status),
        AttrInt('duration_ms', durationMs),
      );
    } catch (err: any) {
      const durationMs = Date.now() - startedAt;
      loza.finishError(ev, err,
        AttrInt('status_code', ctx.status || 500),
        AttrInt('duration_ms', durationMs),
      );
    }
    await loza.emit(ev).catch(() => {});
  };
}
