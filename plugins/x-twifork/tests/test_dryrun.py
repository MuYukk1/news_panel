"""--dry-run 端到端：本地打印校验、state 不落盘、向 mock 核心完成契约推送。"""

import json
import threading

from conftest import CONFIG_EXAMPLE

from mock_core import make_server

from x_twifork.__main__ import main
from x_twifork.transform import validate_heartbeat_payload, validate_ingest_payload


def _run(args, capsys):
    code = main(args)
    out = capsys.readouterr().out
    return code, out


def _parse_blocks(output):
    lines = output.splitlines()
    blocks = {}
    current, buffer = None, []
    for line in lines:
        if line.startswith("--- INGEST") or line.startswith("--- HEARTBEAT"):
            if current:
                blocks[current] = "\n".join(buffer)
            current = "ingest" if line.startswith("--- INGEST") else "heartbeat"
            buffer = []
        elif line.startswith("--- VALIDATION"):
            if current:
                blocks[current] = "\n".join(buffer)
            current, buffer = None, []
            blocks["validation"] = line
        elif current is not None:
            buffer.append(line)
    if current:
        blocks[current] = "\n".join(buffer)
    return blocks


def test_dry_run_print_mode_valid_payloads(tmp_path, capsys):
    code, out = _run(
        ["--dry-run", "--config", str(CONFIG_EXAMPLE), "--state-file", str(tmp_path / "state.json")],
        capsys,
    )
    assert code == 0
    blocks = _parse_blocks(out)
    assert blocks.get("validation") == "--- VALIDATION OK: payloads conform to PLUGIN_SPEC v0.1 ---"

    ingest = json.loads(blocks["ingest"])
    heartbeat = json.loads(blocks["heartbeat"])
    assert validate_ingest_payload(ingest) == []
    assert validate_heartbeat_payload(heartbeat) == []
    assert ingest["source_type"] == "x"
    assert ingest["source_id"] == "x-twifork-main"
    assert len(ingest["items"]) >= 5
    assert heartbeat["status"] == "ok"
    assert heartbeat["plugin_version"]


def test_dry_run_does_not_touch_state_file(tmp_path, capsys):
    state_file = tmp_path / "state.json"
    code, _ = _run(
        ["--dry-run", "--config", str(CONFIG_EXAMPLE), "--state-file", str(state_file)],
        capsys,
    )
    assert code == 0
    assert not state_file.exists()  # dry-run 保持幂等，不写任何状态


def test_dry_run_pushes_to_mock_core(tmp_path, capsys):
    server, base_url = make_server(port=0, token=None)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        code, _ = _run(
            [
                "--dry-run",
                "--config", str(CONFIG_EXAMPLE),
                "--ingest-url", base_url + "/api/v1/ingest",
                "--state-file", str(tmp_path / "state.json"),
            ],
            capsys,
        )
        assert code == 0
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)

    records = server.mock_state.records
    ingest_records = [r for r in records if r["path"] == "/api/v1/ingest"]
    heartbeat_records = [r for r in records if r["path"] == "/api/v1/heartbeat"]
    assert ingest_records and heartbeat_records

    ingest_payload = ingest_records[0]["data"]
    assert ingest_records[0]["auth"].startswith("Bearer ")
    assert validate_ingest_payload(ingest_payload) == []
    assert ingest_payload["items"]
    assert heartbeat_records[0]["data"]["status"] == "ok"
    assert validate_heartbeat_payload(heartbeat_records[0]["data"]) == []


def test_dry_run_works_without_config_file(tmp_path, capsys):
    code, out = _run(["--dry-run", "--state-file", str(tmp_path / "state.json")], capsys)
    assert code == 0
    assert "VALIDATION OK" in out


def test_real_mode_rejects_missing_config(tmp_path):
    code = main(["--config", str(tmp_path / "absent.yaml")])
    assert code == 2
