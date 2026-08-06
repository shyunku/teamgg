import type { ServerResponse } from "node:http";

export function initializeSse(response: ServerResponse, allowOrigin?: string): void {
  response.statusCode = 200;
  response.setHeader("Content-Type", "text/event-stream; charset=utf-8");
  response.setHeader("Cache-Control", "no-cache, no-transform");
  response.setHeader("Connection", "keep-alive");
  response.setHeader("X-Accel-Buffering", "no");
  if (allowOrigin) {
    response.setHeader("Access-Control-Allow-Origin", allowOrigin);
    response.setHeader("Vary", "Origin");
  }
  response.flushHeaders();
}

export function writeSse(response: ServerResponse, event: string, data: unknown): void {
  if (response.destroyed || response.writableEnded) return;
  response.write(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`);
}
