/** Sink interface — destination for encoded events. */
export interface Sink {
  name(): string;
  write(encoded: string): Promise<void> | void;
  flush(): Promise<void> | void;
  close(): Promise<void> | void;
  drain?(): Promise<void> | void;
  pause?(): void;
  resume?(): void;
  queueSize?(): number;
  health?(): boolean | Promise<boolean>;
}
