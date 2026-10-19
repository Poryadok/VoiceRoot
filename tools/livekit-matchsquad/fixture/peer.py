"""Linux-only bounded UDP/TCP attachment probe; this is not a LiveKit worker."""
import json
import os
from pathlib import Path
import selectors
import signal
import socket
import sys
import threading
import time


def write_state(value):
    tmp = Path('/tmp/state.next')
    tmp.write_text(json.dumps(value))
    tmp.replace('/tmp/state.json')


def tcp_worker():
    with socket.socket() as server:
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind(('0.0.0.0', 9002))
        server.listen(4)
        while True:
            connection, _ = server.accept()
            with connection:
                connection.settimeout(1)
                connection.sendall(connection.recv(1024))


def worker():
    # Exercise a child in a different process group. Stopping only the parent
    # is not a whole-sandbox fence; PID-namespace termination must stop both.
    child = os.fork()
    if child:
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
        write_state({'child': child, 'parent': os.getpid()})
        while True:
            time.sleep(1)
    os.setsid()
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    threading.Thread(target=tcp_worker, daemon=True).start()
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as udp:
        udp.setsockopt(socket.SOL_SOCKET, socket.SO_SNDBUF, 16384)
        udp.bind(('0.0.0.0', 9001))
        udp.settimeout(.01)
        peer, nonce, seq = None, b'', 0
        while True:
            try:
                data, address = udp.recvfrom(256)
                if data.startswith(b'START:') and len(data) <= 128:
                    if data == b'START:direct-bypass':
                        Path('/tmp/direct-bypass').write_text('unexpected direct datagram')
                    peer, nonce = address, data[6:]
            except socket.timeout:
                pass
            if peer:
                udp.sendto(b'DATA:' + nonce + b':' + str(seq).encode(), peer)
                seq += 1


def relay(worker_ip):
    # No Docker socket, host networking, NET_ADMIN or alternate destinations.
    # There is exactly one configured upstream attachment, with bounded peers.
    selector = selectors.DefaultSelector()
    front = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    front.setsockopt(socket.SOL_SOCKET, socket.SO_RCVBUF, 16384)
    front.bind(('0.0.0.0', 9001))
    front.setblocking(False)
    selector.register(front, selectors.EVENT_READ, None)
    clients = {}
    stopping = False

    def stop(*_):
        nonlocal stopping
        stopping = True

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)

    def forward_tcp():
        with socket.socket() as server:
            server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            server.bind(('0.0.0.0', 9002))
            server.listen(4)
            server.settimeout(.1)
            while not stopping:
                try:
                    connection, _ = server.accept()
                except socket.timeout:
                    continue
                with connection, socket.create_connection((worker_ip, 9002), timeout=1) as upstream:
                    connection.settimeout(1)
                    upstream.sendall(connection.recv(1024))
                    connection.sendall(upstream.recv(1024))

    threading.Thread(target=forward_tcp, daemon=True).start()
    try:
        while not stopping:
            for key, _ in selector.select(.05):
                if stopping:
                    break
                data, address = key.fileobj.recvfrom(2048)
                if key.fileobj is front:
                    if address not in clients:
                        if len(clients) >= 4:
                            continue
                        upstream = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
                        upstream.setblocking(False)
                        upstream.connect((worker_ip, 9001))
                        clients[address] = upstream
                        selector.register(upstream, selectors.EVENT_READ, address)
                    clients[address].send(data)
                else:
                    front.sendto(data, key.data)
    finally:
        # Close all owned forwarding sockets/queues before acknowledging exit.
        for upstream in clients.values():
            upstream.close()
        front.close()
        selector.close()


def client(edge_ip, worker_ip, nonce):
    state = {'received': 0, 'after_fence': 0, 'tcp_control': False, 'direct_tcp_denied': False, 'direct_udp_denied': False}
    write_state(state)
    # Separate internal networks must block a direct worker path.
    try:
        with socket.create_connection((worker_ip, 9002), timeout=.5):
            raise RuntimeError('direct TCP worker bypass exists')
    except (TimeoutError, OSError):
        state['direct_tcp_denied'] = True
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as direct:
        direct.settimeout(.5)
        try:
            direct.sendto(b'START:direct-bypass', (worker_ip, 9001))
            direct.recvfrom(256)
            raise RuntimeError('direct UDP worker bypass exists')
        except (TimeoutError, OSError):
            state['direct_udp_denied'] = True
    with socket.create_connection((edge_ip, 9002), timeout=2) as connection:
        connection.sendall(b'bounded-tcp-control')
        if connection.recv(256) != b'bounded-tcp-control':
            raise RuntimeError('TCP forwarding control failed')
        state['tcp_control'] = True
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as udp:
        udp.settimeout(.05)
        expected = b'DATA:' + nonce.encode() + b':'
        deadline = time.monotonic() + 45
        while time.monotonic() < deadline:
            udp.sendto(b'START:' + nonce.encode(), (edge_ip, 9001))
            try:
                data, _ = udp.recvfrom(256)
                now = time.monotonic_ns()
                if not data.startswith(expected):
                    raise RuntimeError('foreign probe stream')
                state['received'] += 1
                state['last_received_ns'] = now
                marker = Path('/tmp/fence.marker')
                if marker.exists() and now > int(marker.read_text()):
                    state['after_fence'] += 1
            except (TimeoutError, OSError):
                pass
            write_state(state)
            time.sleep(.01)


if __name__ == '__main__':
    {'worker': worker, 'relay': relay, 'client': client}[sys.argv[1]](*sys.argv[2:])
