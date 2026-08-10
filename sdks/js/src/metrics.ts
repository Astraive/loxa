export interface MetricsSnapshot {
  counters: Record<string, number>;
  gauges: Record<string, number>;
}

export class MetricsCollector {
  private counters: Record<string, number> = {};
  private gauges: Record<string, number> = {};

  inc(name: string, amount = 1): void {
    this.counters[name] = (this.counters[name] ?? 0) + amount;
  }

  setGauge(name: string, value: number): void {
    this.gauges[name] = value;
  }

  snapshot(): MetricsSnapshot {
    return {
      counters: { ...this.counters },
      gauges: { ...this.gauges },
    };
  }

  renderPrometheus(namespace = 'loza_sdk'): string {
    const lines: string[] = [];
    for (const [name, value] of Object.entries(this.counters)) {
      lines.push(`# TYPE ${namespace}_${name} counter`);
      lines.push(`${namespace}_${name} ${value}`);
    }
    for (const [name, value] of Object.entries(this.gauges)) {
      lines.push(`# TYPE ${namespace}_${name} gauge`);
      lines.push(`${namespace}_${name} ${value}`);
    }
    return `${lines.join('\n')}\n`;
  }
}

export function RenderPrometheus(metrics: MetricsCollector): string {
  return metrics.renderPrometheus();
}
