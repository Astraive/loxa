import { Logger } from '../core/logger.ts';
import { String as AttrString, Int as AttrInt } from '../core/event.ts';

export interface NextMiddlewareOptions {
  service?: string;
}

export function loxaNextMiddleware(opts: NextMiddlewareOptions = {}) {
  const loxa = new Logger({ service: opts.service });

  return async (request: any, event: any) => {
    const startedAt = Date.now();
    const route = request.nextUrl?.pathname || request.url || '';

    const ev = loxa.startHTTPEvent({
      event: `${request.method || 'GET'} ${route}`,
      kind: 'http',
      method: request.method || 'GET',
      path: route,
      route,
      service: opts.service,
    });

    loxa.enrich(ev,
      AttrString('http.user_agent', request.headers?.get('user-agent') || ''),
      AttrString('http.remote_ip', request.headers?.get('x-forwarded-for') || ''),
    );

    try {
      const response = await event();
      const durationMs = Date.now() - startedAt;
      const outcome = response.status >= 500 ? 'error' : 'success';
      loxa.finish(ev, outcome,
        AttrInt('status_code', response.status),
        AttrInt('duration_ms', durationMs),
      );
      await loxa.emit(ev).catch(() => {});
      return response;
    } catch (err: any) {
      loxa.finishError(ev, err);
      await loxa.emit(ev).catch(() => {});
      throw err;
    }
  };
}
