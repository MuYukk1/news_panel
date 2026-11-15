#!/usr/bin/env python3
"""news_panel 核心服务本地 Mock（仅标准库，无第三方依赖）。

按 docs/PLUGIN_SPEC.md (v0.1) 模拟 POST /api/v1/ingest 与 POST /api/v1/heartbeat：
- 校验 Bearer Token（--token 指定时）与顶层必填字段；
- 以 (source_id, external_id) 模拟幂等去重并返回 accepted/deduplicated 计数；
- --fail-mode 401/429/500 便于实测插件侧退避重试与心跳上报；
- --dump 将每次收到的 ingest/heartbeat 请求落盘为 JSONL 供人工核查。

用法（插件独立自测，无需 Go 核心先就绪）::

    python tools/mock_core.py --port 8099 --token mock-token --dump mock_traffic.jsonl

也可被 pytest 导入：``make_server(port=0, token=...) -> (server, base_url)``
"""

from __future__ import annotations

import argparse
import json
import sys
import threading
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit


def utcnow_rfc3339() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


class MockState:
    """每个 mock 服务实例的共享状态（线程安全）。"""

    def __init__(
        self,
        token: str | None = None,
        fail_mode: str | None = None,
        verbose: bool = False,
    ) -> None:
        self.token = token
        self.fail_mode = fail_mode  # None | "401" | "429" | "500"
        self.verbose = verbose
        self.lock = threading.Lock()
        self.seen: set[tuple[str, str]] = set()   # (source_id, external_id)
        self.records: list[dict] = []             # 每次收到的 ingest/heartbeat 请求


def make_handler(state: MockState):
    class MockCoreHandler(BaseHTTPRequestHandler):
        server_version = "MockNewsPanelCore/0.1"
        protocol_version = "HTTP/1.1"

        # ---------- 帮助方法 ----------
        def _send_json(self, status: int, payload: dict) -> None:
            body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def _record(self, path: str, data: dict, auth: str) -> None:
            with state.lock:
                state.records.append({"path": path, "auth": auth, "data": data})

        # ---------- HTTP ----------
        def do_POST(self) -> None:  # noqa: N802 —— http.server API
            length = int(self.headers.get("Content-Length") or 0)
            raw = self.rfile.read(length) if length else b""
            auth = self.headers.get("Authorization", "")

            if state.fail_mode == "401":
                self._send_json(401, {"error": "unauthorized (fail-mode)"})
                return
            expected = f"Bearer {state.token}" if state.token else None
            if expected and auth != expected:
                self._send_json(401, {"error": "unauthorized"})
                return
            if state.fail_mode == "429":
                self._send_json(429, {"error": "slow down (fail-mode)"})
                return
            if state.fail_mode == "500":
                self._send_json(500, {"error": "internal error (fail-mode)"})
                return

            try:
                data = json.loads(raw.decode("utf-8")) if raw else None
            except (UnicodeDecodeError, json.JSONDecodeError):
                self._send_json(400, {"error": "request body is not valid JSON"})
                return

            path = urlsplit(self.path).path
            if path == "/api/v1/ingest":
                self._handle_ingest(data, auth)
            elif path == "/api/v1/heartbeat":
                self._record(path, data or {}, auth)
                self._send_json(200, {"status": "ok", "server_time": utcnow_rfc3339()})
            else:
                self._send_json(404, {"error": "not found", "path": path})

        # ---------- 端点 ----------
        def _handle_ingest(self, data, auth: str) -> None:
            if not isinstance(data, dict):
                self._send_json(400, {"error": "request body must be a JSON object"})
                return
            for key in ("source_id", "source_type", "items"):
                if key not in data:
                    self._send_json(400, {"error": f"missing required top-level field: {key}"})
                    return
            items = data["items"]
            if not isinstance(items, list) or not 1 <= len(items) <= 100:
                self._send_json(400, {"error": "items must be an array of 1..100"})
                return
            accepted = deduplicated = 0
            with state.lock:
                for item in items:
                    if not isinstance(item, dict):
                        continue
                    key = (str(data["source_id"]), str(item.get("external_id", "")))
                    if key in state.seen:
                        deduplicated += 1
                    else:
                        state.seen.add(key)
                        accepted += 1
            self._record("/api/v1/ingest", data, auth)
            self._send_json(
                200,
                {"accepted": accepted, "deduplicated": deduplicated, "rejected": 0, "errors": []},
            )

        def log_message(self, fmt: str, *args) -> None:  # noqa: N802 —— http.server API
            if state.verbose:
                sys.stderr.write(f"[mock-core] {self.address_string()} {fmt % args}\n")

    return MockCoreHandler


def make_server(
    port: int = 0,
    token: str | None = None,
    fail_mode: str | None = None,
    verbose: bool = False,
):
    """供 pytest 复用：绑定 127.0.0.1 随机端口，返回 (server, base_url)。"""
    state = MockState(token=token, fail_mode=fail_mode, verbose=verbose)
    server = ThreadingHTTPServer(("127.0.0.1", port), make_handler(state))
    server.mock_state = state  # type: ignore[attr-defined]
    return server, f"http://127.0.0.1:{server.server_address[1]}"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Mock news_panel core for standalone plugin testing (PLUGIN_SPEC v0.1)"
    )
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8099)
    parser.add_argument("--token", default=None, help="require exactly this Bearer token (omit: accept any)")
    parser.add_argument(
        "--fail-mode", choices=["401", "429", "500"], default=None,
        help="force deterministic failure responses to test plugin backoff",
    )
    parser.add_argument("--dump", default=None, help="append every received request body to this JSONL file")
    args = parser.parse_args(argv)

    state = MockState(token=args.token, fail_mode=args.fail_mode, verbose=True)
    server = ThreadingHTTPServer((args.host, args.port), make_handler(state))
    server.mock_state = state  # type: ignore[attr-defined]
    base = f"http://{args.host}:{server.server_address[1]}"
    print(
        f"[mock-core] listening on {base}  (token required: {bool(args.token)}, fail-mode: {args.fail_mode})",
        flush=True,
    )

    stop = threading.Event()

    def _dumper() -> None:
        written = 0
        while not stop.is_set():
            with state.lock:
                pending = state.records[written:]
                written = len(state.records)
            if pending:
                with open(args.dump, "a", encoding="utf-8") as handle:
                    for record in pending:
                        handle.write(json.dumps(record, ensure_ascii=False) + "\n")
            stop.wait(0.5)

    if args.dump:
        threading.Thread(target=_dumper, daemon=True).start()

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n[mock-core] shutting down", flush=True)
    finally:
        stop.set()
        server.server_close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
