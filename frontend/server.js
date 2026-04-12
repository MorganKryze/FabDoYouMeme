// Custom Node entrypoint that wraps SvelteKit's adapter-node `handler` and
// adds a WebSocket proxy for /api/ws/* → backend. The HTTP /api/* proxy
// lives in `src/hooks.server.ts` (SvelteKit's `handle` hook can see HTTP
// requests), but `handle` never runs for WebSocket upgrades — those are
// emitted as a separate `upgrade` event on the underlying http.Server.
// So we tunnel upgrade requests at the raw-socket layer here.
//
// This mirrors the production reverse-proxy topology in dev where the
// backend container port is never published to the host: the browser
// opens `ws://<frontend-origin>/api/ws/...` and this server relays the
// handshake (and the subsequent duplex traffic) to `backend:8080`.

import http from 'node:http';
import process from 'node:process';
import { handler } from './build/handler.js';

const PORT = Number(process.env.PORT ?? 3000);
const HOST = process.env.HOST ?? '0.0.0.0';
const API = new URL(process.env.API_URL ?? 'http://backend:8080');

const server = http.createServer(handler);

server.on('upgrade', (clientReq, clientSocket, clientHead) => {
  if (!clientReq.url || !clientReq.url.startsWith('/api/ws/')) {
    clientSocket.destroy();
    return;
  }

  const outHeaders = { ...clientReq.headers };
  delete outHeaders.host;

  const proxyReq = http.request({
    hostname: API.hostname,
    port: API.port || (API.protocol === 'https:' ? 443 : 80),
    method: 'GET',
    path: clientReq.url,
    headers: outHeaders,
  });

  proxyReq.on('response', (proxyRes) => {
    // Backend chose NOT to upgrade — e.g. RequireAuth rejected the
    // handshake with 401 before the websocket handler ran. Relay the
    // plain HTTP response so the client sees a real status/body instead
    // of a hung socket.
    const statusLine = `HTTP/1.1 ${proxyRes.statusCode} ${proxyRes.statusMessage ?? ''}\r\n`;
    const raw = proxyRes.rawHeaders;
    const lines = [];
    for (let i = 0; i < raw.length; i += 2) {
      if (raw[i].toLowerCase() === 'transfer-encoding') continue;
      lines.push(`${raw[i]}: ${raw[i + 1]}`);
    }
    lines.push('Connection: close');
    clientSocket.write(statusLine + lines.join('\r\n') + '\r\n\r\n');
    proxyRes.pipe(clientSocket);
    proxyRes.on('end', () => clientSocket.end());
  });

  proxyReq.on('upgrade', (proxyRes, proxySocket, proxyHead) => {
    // Replay the backend's 101 Switching Protocols verbatim, preserving
    // header case via rawHeaders (Sec-WebSocket-Accept hashing is header
    // name case-insensitive per RFC 6455 but some client libraries are
    // strict — rawHeaders keeps the on-the-wire representation identical).
    const statusLine = `HTTP/1.1 ${proxyRes.statusCode} ${proxyRes.statusMessage}\r\n`;
    const lines = [];
    const raw = proxyRes.rawHeaders;
    for (let i = 0; i < raw.length; i += 2) {
      lines.push(`${raw[i]}: ${raw[i + 1]}`);
    }
    clientSocket.write(statusLine + lines.join('\r\n') + '\r\n\r\n');
    if (proxyHead.length) clientSocket.write(proxyHead);

    proxySocket.pipe(clientSocket);
    clientSocket.pipe(proxySocket);

    const cleanup = () => {
      proxySocket.destroy();
      clientSocket.destroy();
    };
    proxySocket.on('error', cleanup);
    clientSocket.on('error', cleanup);
    proxySocket.on('close', cleanup);
    clientSocket.on('close', cleanup);
  });

  proxyReq.on('error', (err) => {
    console.error('[ws-proxy] backend error', err);
    clientSocket.destroy();
  });

  if (clientHead.length) proxyReq.write(clientHead);
  proxyReq.end();
});

const shutdown = () => {
  server.close(() => process.exit(0));
  setTimeout(() => process.exit(1), 5000).unref();
};
process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);

server.listen(PORT, HOST, () => {
  console.log(`Listening on http://${HOST}:${PORT}`);
});
