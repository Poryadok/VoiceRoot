"""Private test-only Service forwarding: ordinary routes require application health.

The peer route exposes only the real User transport. This models the selected
ClusterIP/EndpointSlice contract; it does not claim an actual Kubernetes rollout.
"""
import select
import socket
import socketserver
import threading
import urllib.request


def healthy(host):
    try:
        with urllib.request.urlopen('http://' + host + ':8080/health', timeout=0.5) as response:
            return response.status == 200
    except Exception:
        return False


class Forward(socketserver.BaseRequestHandler):
    def handle(self):
        target, gated = self.server.target, self.server.gated
        if gated and not healthy(target):
            return
        try:
            upstream = socket.create_connection((target, 9090), timeout=0.5)
        except OSError:
            return
        with upstream:
            pair = [self.request, upstream]
            while True:
                readable, _, _ = select.select(pair, [], [], 1)
                for source in readable:
                    try:
                        data = source.recv(65536)
                        if not data:
                            return
                        (upstream if source is self.request else self.request).sendall(data)
                    except OSError:
                        return


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


servers = []
for port, target, gated in [(19090, 'user', True), (19091, 'space', True), (19092, 'user', False)]:
    server = Server(('0.0.0.0', port), Forward)
    server.target, server.gated = target, gated
    servers.append(server)
    threading.Thread(target=server.serve_forever, daemon=True).start()
print('COLDSTART_FORWARDING_READY normal_user=health normal_space=health peer_user=transport_only', flush=True)
threading.Event().wait()
