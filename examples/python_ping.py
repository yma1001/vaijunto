#!/usr/bin/env python3
"""Cliente mínimo em Python 3 (stdlib) para o protocolo VAIJUNTO.

Prova que o framing + JSON não dependem de Go: um programador consegue
implementar o cliente só com docs/PROTOCOL.md.
"""
import json
import socket
import struct
import sys

HOST = sys.argv[1] if len(sys.argv) > 1 else "127.0.0.1"
PORT = int(sys.argv[2] if len(sys.argv) > 2 else "5000")


def write_frame(sock, payload: bytes) -> None:
    sock.sendall(struct.pack(">I", len(payload)) + payload)


def read_frame(sock) -> bytes:
    header = recvall(sock, 4)
    n = struct.unpack(">I", header)[0]
    if n == 0 or n > 1024 * 1024:
        raise RuntimeError(f"invalid frame length {n}")
    return recvall(sock, n)


def recvall(sock, n: int) -> bytes:
    buf = bytearray()
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if not chunk:
            raise RuntimeError("incomplete frame")
        buf.extend(chunk)
    return bytes(buf)


def call(sock, operation, request_id, data=None):
    req = {
        "version": 1,
        "operation": operation,
        "requestId": request_id,
        "data": data or {},
    }
    write_frame(sock, json.dumps(req).encode("utf-8"))
    resp = json.loads(read_frame(sock).decode("utf-8"))
    return resp


def main():
    with socket.create_connection((HOST, PORT), timeout=10) as sock:
        pong = call(sock, "PING", "py-1")
        print(json.dumps(pong, indent=2, ensure_ascii=False))
        if pong.get("status") != "OK":
            sys.exit(1)
        print("PYTHON CLIENT PING OK")


if __name__ == "__main__":
    main()
