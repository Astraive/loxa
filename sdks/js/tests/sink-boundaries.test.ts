import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import {
  StdoutSink, StderrSink, FileSink, RotatingFileSink, NoopSink, MemorySink,
  HTTPBatchSink, CollectorSink, MultiSink, OtlpSink,
  stdoutSink, stderrSink, noopSink, memorySink, fileSink, rotatingFileSink,
  collectorSink, httpBatchSink, multiSink, otlpSink, drain, pause, resume,
  queueSize, health, kafkaSink, httpSink,
} from '../src/sinks/standard-sinks.ts';

const acceptedResponse = JSON.stringify({ request_id: 'req-1', status: 'accepted', accepted: 1, rejected: 0, invalid: 0, acks: [] });

async function listenServer(handler: (req: http.IncomingMessage, res: http.ServerResponse) => void): Promise<{ server: http.Server; endpoint: string }> {
  const server = http.createServer(handler);
  const gate = Promise.withResolvers<void>();
  server.once('error', gate.reject);
  server.listen(0, '127.0.0.1', () => gate.resolve());
  await gate.promise;
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('server did not expose a port');
  return { server, endpoint: `http://127.0.0.1:${address.port}/events` };
}

async function closeServer(server: http.Server): Promise<void> {
  const gate = Promise.withResolvers<void>();
  server.close(error => error ? gate.reject(error) : gate.resolve());
  await gate.promise;
}

describe('Sink boundaries', () => {
  it('writes to stdout, stderr, files, rotating files, and memory', () => {
    const stdout = new StdoutSink();
    const stderr = new StderrSink();
    assert.equal(stdout.name(), 'stdout');
    assert.equal(stderr.name(), 'stderr');
    stdout.flush();
    stdout.write('stdout-test');
    stderr.write('stderr-test');
    stdout.close();
    stderr.flush();
    stderr.close();

    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'loza-sink-'));
    const filePath = path.join(tempDir, 'events.ndjson');
    try {
      const file = new FileSink(filePath);
      assert.equal(file.name(), 'file');
      file.write('one');
      file.flush();
      file.close();
      assert.equal(fs.readFileSync(filePath, 'utf8'), 'one\n');
      const rotatingPath = path.join(tempDir, 'rotating.ndjson');
      fs.writeFileSync(`${rotatingPath}.old1`, 'old1');
      fs.writeFileSync(`${rotatingPath}.old2`, 'old2');
      const rotating = new RotatingFileSink(rotatingPath, 5, 1);
      rotating.write('1234');
      rotating.write('5678');
      rotating.flush();
      rotating.close();
      assert.equal(rotating.name(), 'rotating_file');
      const factoryFile = fileSink(path.join(tempDir, 'factory.ndjson'));
      factoryFile.write('factory');
      rotatingFileSink(path.join(tempDir, 'factory-rotating.ndjson'), 100, 1).write('factory');
      const noop = new NoopSink();
      noop.write('ignored');
      noop.flush();
      noop.close();
      assert.equal(noop.name(), 'noop');
      const memory = new MemorySink();
      memory.write('one');
      assert.deepEqual(memory.getEvents(), ['one']);
      assert.equal(memory.getLength(), 1);
      memory.clear();
      assert.equal(memory.getLength(), 0);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it('sends compressed JSON batches and reports acknowledgements', async () => {
    let bodyBytes = 0;
    const received = Promise.withResolvers<void>();
    const { server, endpoint } = await listenServer((req, res) => {
      req.on('data', chunk => { bodyBytes += chunk.length; });
      req.on('end', () => {
        res.statusCode = 201;
        res.setHeader('content-type', 'application/json');
        res.end(acceptedResponse);
        received.resolve();
      });
    });
    try {
      const acks: string[] = [];
      const sink = new HTTPBatchSink({ endpoint, apiKey: 'key', batchSize: 2, flushIntervalMs: 1000, statsHandler: {
        onCollectorAck(data) { acks.push(data.requestId); },
      } });
      assert.equal(sink.name(), 'httpbatch');
      await sink.write('{"event":"one"}');
      assert.equal(sink.queueSize(), 1);
      await sink.write('{"event":"two"}');
      await received.promise;
      assert.equal(bodyBytes > 0, true);
      assert.equal(sink.queueSize(), 0);
      assert.equal(sink.lastCollectorResponse?.status, 'accepted');
      assert.deepEqual(acks, ['req-1']);
      await sink.drain();
      sink.pause();
      sink.resume();
      sink.close();
    } finally {
      await closeServer(server);
    }
  });

  it('sends NDJSON batches without compression and preserves events on failures', async () => {
    let requests = 0;
    const { server, endpoint } = await listenServer((_req, res) => {
      requests++;
      res.statusCode = 400;
      res.end('not-json');
    });
    try {
      const sink = new HTTPBatchSink({ endpoint, enableCompression: false, ndjson: true, retries: 0, batchSize: 1 });
      await assert.rejects(() => sink.write('{"event":"bad"}'), /collector reported batch failure/);
      assert.equal(requests, 1);
      assert.equal(sink.queueSize(), 1);
      await assert.rejects(() => sink.flush(), /collector reported batch failure/);
    } finally {
      await closeServer(server);
    }
  });

  it('retries retryable responses and handles callback failures', async () => {
    let requests = 0;
    const { server, endpoint } = await listenServer((_req, res) => {
      requests++;
      if (requests === 1) {
        res.statusCode = 503;
        res.setHeader('retry-after', '0');
        res.end(JSON.stringify({ request_id: 'retry', status: 'accepted', accepted: 0, rejected: 0, invalid: 0, acks: [] }));
      } else {
        res.statusCode = 200;
        res.end(acceptedResponse);
      }
    });
    try {
      const sink = new HTTPBatchSink({ endpoint, retries: 1, baseDelay: 0, maxDelay: 0, batchSize: 1, statsHandler: {
        onCollectorAck() { throw new Error('stats failure'); },
      } });
      await sink.write('{"event":"retry"}');
      assert.equal(requests, 2);
      assert.equal(sink.queueSize(), 0);
    } finally {
      await closeServer(server);
    }
  });

  it('maps transport failures and response-level retryable errors', async () => {
    const unreachable = new HTTPBatchSink({ endpoint: 'http://127.0.0.1:1/events', retries: 0, batchSize: 1, timeout: 10 });
    await assert.rejects(() => unreachable.write('{"event":"transport"}'), /collector send failed/);
    const { server, endpoint } = await listenServer((_req, res) => {
      res.statusCode = 200;
      res.end(JSON.stringify({ request_id: 'retryable', status: 'accepted', accepted: 1, rejected: 0, invalid: 0, acks: [{ event_id: 'id', status: 'retry', retryable: true }] }));
    });
    try {
      const sink = new HTTPBatchSink({ endpoint, retries: 0, batchSize: 1 });
      await assert.rejects(() => sink.write('{"event":"retryable"}'), /collector reported retryable errors/);
      assert.equal(sink.queueSize(), 1);
    } finally {
      await closeServer(server);
    }
  });

  it('covers timer-driven flush, pause/resume cleanup, Basic auth, and OTLP delegation', async () => {
    const { server, endpoint } = await listenServer((_req, res) => {
      res.statusCode = 200;
      res.end(acceptedResponse);
    });
    try {
      const sink = new HTTPBatchSink({ endpoint, username: 'user', password: 'secret', batchSize: 10, flushIntervalMs: 1 });
      await sink.write('{"event":"timer"}');
      await new Promise(resolve => setTimeout(resolve, 5));
      sink.pause();
      sink.resume();
      await sink.flush();
      const otlp = new OtlpSink(endpoint);
      await otlp.write('{"event":"otlp"}');
      await otlp.flush();
      await otlp.close();
    } finally {
      await closeServer(server);
    }
  });

  it('classifies permanent and partially accepted response statuses', async () => {
    let request = 0;
    const { server, endpoint } = await listenServer((_req, res) => {
      request++;
      res.statusCode = 200;
      res.end(JSON.stringify(request === 1
        ? { request_id: 'partial', status: 'rejected', accepted: 1, rejected: 1, invalid: 0, acks: [] }
        : { request_id: 'invalid', status: 'invalid', accepted: 0, rejected: 1, invalid: 1, acks: [] }));
    });
    try {
      const sink = new HTTPBatchSink({ endpoint, retries: 0, batchSize: 1, enableCompression: false });
      await assert.rejects(() => sink.write('{"event":"partial"}'), /retryable errors/);
      await assert.rejects(() => sink.write('{"event":"invalid"}'), /batch failure/);
    } finally {
      await closeServer(server);
    }
  });

  it('covers HTTP response fallbacks, retry delay branches, and pending timer controls', async () => {
    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'loza-sink-default-'));
    try {
      const rotating = new RotatingFileSink(path.join(tempDir, 'default.ndjson'));
      rotating.write('default');
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
    let request = 0;
    const { server, endpoint } = await listenServer((_req, res) => {
      request++;
      if (request === 1) {
        res.statusCode = 503;
        res.end(JSON.stringify({ request_id: '', status: 'accepted', accepted: 0, rejected: 0, invalid: 0 }));
      } else if (request === 2) {
        res.statusCode = 200;
        res.end('invalid');
      } else {
        res.statusCode = 200;
        res.end(JSON.stringify({ status: 'accepted', accepted: 1, rejected: 0, invalid: 0 }));
      }
    });
    try {
      const sink = new HTTPBatchSink({ endpoint, retries: 1, baseDelay: 0, maxDelay: 0, batchSize: 1, enableCompression: false });
      await sink.write('{"event":"retry-no-header"}');
      await sink.write('{"event":"malformed-success"}');
      const pending = new HTTPBatchSink({ endpoint, batchSize: 10, flushIntervalMs: 10000, enableCompression: false });
      await pending.write('{"event":"pending"}');
      pending.pause();
      pending.resume();
      pending.close();
      assert.equal(pending.queueSize(), 1);
    } finally {
      await closeServer(server);
    }
  });

  it('validates HTTP options and exposes wrappers and helper branches', () => {
    assert.throws(() => new HTTPBatchSink({ endpoint: 'https://user:pass@example.com/events' }), /credentials/);
    assert.throws(() => new HTTPBatchSink({ endpoint: 'https://example.com/events', password: 'secret' }), /password requires/);
    assert.throws(() => new HTTPBatchSink({ endpoint: 'https://example.com/events', username: 'private' }), /require a password/);
    assert.throws(() => new HTTPBatchSink({ endpoint: 'http://example.com/events', username: 'user', password: 'secret' }), /require HTTPS/);
    assert.doesNotThrow(() => new HTTPBatchSink({ endpoint: 'http://localhost/events', username: 'user', password: 'secret' }));
    assert.doesNotThrow(() => new HTTPBatchSink({ endpoint: 'https://example.com/events', username: 'lx_pub_capability' }));
    assert.ok(collectorSink({ endpoint: 'http://localhost/events' }) instanceof CollectorSink);
    assert.ok(httpBatchSink({ endpoint: 'http://localhost/events' }) instanceof HTTPBatchSink);
    assert.ok(httpSink({ endpoint: 'http://localhost/events' }) instanceof HTTPBatchSink);
    assert.ok(kafkaSink().name());
    assert.ok(kafkaSink({ endpoint: 'http://localhost/custom' }).name());
    assert.ok(otlpSink() instanceof OtlpSink);
    assert.ok(otlpSink('http://localhost/custom') instanceof OtlpSink);
    assert.ok(stdoutSink() instanceof StdoutSink);
    assert.ok(stderrSink() instanceof StderrSink);
    assert.ok(noopSink() instanceof NoopSink);
    assert.ok(memorySink() instanceof MemorySink);

    let writes = 0;
    let flushes = 0;
    let closes = 0;
    const sink = {
      name: () => 'custom',
      write: () => { writes++; },
      flush: () => { flushes++; },
      close: () => { closes++; },
      queueSize: () => 2,
    };
    const other = new MemorySink();
    const multi = multiSink(sink, other);
    assert.equal(multi.name(), 'multi');
    return Promise.all([
      multi.write('event'),
      multi.flush(),
      multi.close(),
      drain(sink),
    ]).then(() => {
      assert.equal(writes, 1);
      assert.equal(flushes, 2);
      assert.equal(closes, 1);
      pause({});
      resume({});
      assert.equal(queueSize(sink), 2);
      assert.equal(queueSize({ getLength: () => 3 }), 3);
      assert.equal(queueSize({}), 0);
      assert.deepEqual(health(sink), { name: 'custom', status: 'healthy' });
    });
  });
});
