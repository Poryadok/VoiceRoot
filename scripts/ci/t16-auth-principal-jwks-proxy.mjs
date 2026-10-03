import fs from 'node:fs';
import http from 'node:http';
import https from 'node:https';

const route = '/api/v1/auth/.well-known/principal-jwks.json';
const server = https.createServer({
  key: fs.readFileSync('/run/t16/auth-principal-jwks.key'),
  cert: fs.readFileSync('/run/t16/auth-principal-jwks.crt'),
  minVersion: 'TLSv1.2',
}, (request, response) => {
  if (request.method === 'GET' && request.url === '/health') {
    response.writeHead(200, { 'content-type': 'text/plain', 'cache-control': 'no-store' });
    response.end('ok');
    return;
  }
  if (request.method !== 'GET' || request.url !== route) {
    response.writeHead(404, { 'cache-control': 'no-store' });
    response.end();
    return;
  }

  const upstream = http.request({
    hostname: '127.0.0.1',
    port: 8080,
    path: route,
    method: 'GET',
    headers: { accept: 'application/json' },
    timeout: 2000,
  }, (result) => {
    response.writeHead(result.statusCode ?? 502, {
      'content-type': result.headers['content-type'] ?? 'application/json',
      'cache-control': 'no-store',
    });
    result.pipe(response);
  });
  upstream.on('timeout', () => upstream.destroy(new Error('Auth JWKS upstream timed out')));
  upstream.on('error', () => {
    if (!response.headersSent) response.writeHead(502, { 'cache-control': 'no-store' });
    response.end();
  });
  upstream.end();
});

server.listen(9444, '0.0.0.0');
