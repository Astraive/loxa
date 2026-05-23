import { Logger } from '../core/logger.ts';
import { String as AttrString, Int as AttrInt } from '../core/event.ts';

export interface FastifyPluginOptions {
  service?: string;
}

export function loxaFastifyPlugin(opts: FastifyPluginOptions = {}) {
  const loxa = new Logger({ service: opts.service });

  return async (request: any, reply: any) => {
    const startedAt = Date.now();
    const route = request.routeOptions?.url || request.url || '';

    const ev = loxa.startHTTPEvent({
      event: `${request.method} ${route}`,
      kind: 'http',
      method: request.method,
      path: request.url,
      route,
      service: opts.service,
    });

    loxa.enrich(ev,
      AttrString('http.user_agent', request.headers?.['user-agent'] || ''),
      AttrString('http.remote_ip', request.ip || request.socket?.remoteAddress || ''),
    );

    reply.then?.(
      () => {
        const durationMs = Date.now() - startedAt;
        const outcome = reply.statusCode >= 500 ? 'error' : 'success';
        loxa.finish(ev, outcome,
          AttrInt('status_code', reply.statusCode),
          AttrInt('duration_ms', durationMs),
        );
        loxa.emit(ev).catch(() => {});
      },
      (err: any) => {
        loxa.finishError(ev, err);
        loxa.emit(ev).catch(() => {});
      },
    );
  };
}
