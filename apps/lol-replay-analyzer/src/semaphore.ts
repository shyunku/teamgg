import { throwIfAborted } from "./errors.js";

interface Waiter {
  resolve: (release: () => void) => void;
  reject: (error: unknown) => void;
  signal: AbortSignal;
  onAbort: () => void;
}

export class Semaphore {
  private active = 0;
  private readonly queue: Waiter[] = [];

  constructor(private readonly capacity: number) {
    if (!Number.isInteger(capacity) || capacity < 1) {
      throw new Error("Semaphore capacity must be a positive integer");
    }
  }

  get pending(): number {
    return this.queue.length;
  }

  get running(): number {
    return this.active;
  }

  async acquire(signal: AbortSignal): Promise<() => void> {
    throwIfAborted(signal);
    if (this.active < this.capacity) {
      this.active += 1;
      return this.createRelease();
    }

    return new Promise<() => void>((resolve, reject) => {
      const waiter: Waiter = {
        resolve,
        reject,
        signal,
        onAbort: () => {
          const index = this.queue.indexOf(waiter);
          if (index >= 0) this.queue.splice(index, 1);
          reject(signal.reason ?? new Error("Analysis aborted while queued"));
        },
      };
      signal.addEventListener("abort", waiter.onAbort, { once: true });
      this.queue.push(waiter);
    });
  }

  private createRelease(): () => void {
    let released = false;
    return () => {
      if (released) return;
      released = true;
      this.active -= 1;
      this.dispatch();
    };
  }

  private dispatch(): void {
    while (this.active < this.capacity && this.queue.length > 0) {
      const waiter = this.queue.shift();
      if (!waiter) return;
      waiter.signal.removeEventListener("abort", waiter.onAbort);
      if (waiter.signal.aborted) {
        waiter.reject(waiter.signal.reason ?? new Error("Analysis aborted while queued"));
        continue;
      }
      this.active += 1;
      waiter.resolve(this.createRelease());
    }
  }
}
