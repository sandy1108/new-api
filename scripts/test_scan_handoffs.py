#!/usr/bin/env python3
"""交接包扫描器的行为测试。"""

from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("scan-handoffs.py")
SPEC = importlib.util.spec_from_file_location("scan_handoffs", SCRIPT_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"无法加载扫描器：{SCRIPT_PATH}")
SCAN_HANDOFFS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SCAN_HANDOFFS)


class ScanHandoffsTest(unittest.TestCase):
    def write_metadata(self, package_dir: Path, **overrides: object) -> None:
        metadata = {
            "handoff_id": "demo-handoff",
            "title": "示例交接包",
            "status": "pending",
            "sender_agent": "codex",
            "executor_agent": "doubao",
            "created_at": "2026-08-31",
            "started_at": None,
            "completed_at": None,
            "result": None,
            "handoff_file": "HANDOFF.md",
            "feedback_file": "EXECUTION_FEEDBACK.md",
            "sensitivity": "private",
        }
        metadata.update(overrides)
        (package_dir / "handoff.json").write_text(
            json.dumps(metadata, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )

    def test_discovers_standard_backup_and_legacy_packages(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir) / "new-api-development"
            standard = root / "docs/development/handoffs/standard"
            backup = Path(temp_dir) / ".backups/new-api/handoffs/backup"
            legacy = Path(temp_dir) / ".backups/new-api/pre-switch-20260831-rc29"
            for package in (standard, backup, legacy):
                package.mkdir(parents=True)

            self.write_metadata(
                standard,
                handoff_id="standard-handoff",
                title="标准交接包",
                status="completed",
                completed_at="2026-08-31T10:00:00+08:00",
                result="completed",
            )
            (standard / "HANDOFF.md").write_text("# standard\n", encoding="utf-8")
            (standard / "EXECUTION_FEEDBACK.md").write_text("# feedback\n", encoding="utf-8")

            self.write_metadata(
                backup,
                handoff_id="backup-handoff",
                title="备份交接包",
            )
            (backup / "HANDOFF.md").write_text("# backup\n", encoding="utf-8")

            (legacy / "PRODUCTION_SWITCH_HANDOFF.md").write_text(
                "# legacy package\n", encoding="utf-8"
            )

            report = SCAN_HANDOFFS.scan_handoffs(root)

            self.assertEqual(report["counts"]["total"], 3)
            self.assertEqual(report["counts"]["completed"], 1)
            self.assertEqual(report["counts"]["pending"], 2)
            records = {item["handoff_id"]: item for item in report["handoffs"]}
            self.assertEqual(records["standard-handoff"]["source"], "project")
            self.assertEqual(records["backup-handoff"]["source"], "backup")
            self.assertTrue(records["standard-handoff"]["feedback_exists"])
            self.assertTrue(records["legacy-pre-switch-20260831-rc29"]["legacy"])
            self.assertEqual(
                records["legacy-pre-switch-20260831-rc29"]["status"], "pending"
            )
            self.assertTrue(Path(records["backup-handoff"]["package_dir"]).is_absolute())

    def test_invalid_metadata_is_reported_without_stopping_scan(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir) / "new-api-development"
            package = root / "docs/development/handoffs/invalid"
            package.mkdir(parents=True)
            self.write_metadata(package, status="not-a-status")
            (package / "HANDOFF.md").write_text("# invalid\n", encoding="utf-8")

            report = SCAN_HANDOFFS.scan_handoffs(root)

            self.assertEqual(report["counts"]["total"], 1)
            self.assertEqual(report["counts"]["invalid"], 1)
            self.assertEqual(report["handoffs"][0]["status"], "invalid")
            self.assertIn("status", " ".join(report["handoffs"][0]["errors"]))

    def test_non_string_status_is_reported_as_invalid(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir) / "new-api-development"
            package = root / "docs/development/handoffs/invalid-type"
            package.mkdir(parents=True)
            self.write_metadata(package, status=["completed"])
            (package / "HANDOFF.md").write_text("# invalid type\n", encoding="utf-8")

            report = SCAN_HANDOFFS.scan_handoffs(root)

            self.assertEqual(report["handoffs"][0]["status"], "invalid")
            self.assertEqual(report["counts"]["invalid"], 1)

    def test_standard_package_without_metadata_is_visible_as_invalid(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir) / "new-api-development"
            package = root / "docs/development/handoffs/missing-metadata"
            package.mkdir(parents=True)
            (package / "HANDOFF.md").write_text("# missing metadata\n", encoding="utf-8")

            report = SCAN_HANDOFFS.scan_handoffs(root)

            self.assertEqual(report["counts"]["total"], 1)
            self.assertEqual(report["handoffs"][0]["status"], "invalid")
            self.assertIn("metadata file is missing", report["handoffs"][0]["errors"])

    def test_invalid_utf8_metadata_is_reported_without_stopping_scan(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir) / "new-api-development"
            package = root / "docs/development/handoffs/invalid-encoding"
            package.mkdir(parents=True)
            (package / "handoff.json").write_bytes(b"{\xff\n")
            (package / "HANDOFF.md").write_text("# invalid encoding\n", encoding="utf-8")

            report = SCAN_HANDOFFS.scan_handoffs(root)

            self.assertEqual(report["counts"]["invalid"], 1)
            self.assertEqual(report["handoffs"][0]["status"], "invalid")

    def test_unreadable_legacy_metadata_is_invalid_not_inferred_pending(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir) / "new-api-development"
            package = Path(temp_dir) / ".backups/new-api/pre-switch-bad-metadata"
            package.mkdir(parents=True)
            (package / "handoff.json").write_bytes(b"{\xff\n")
            (package / "PRODUCTION_SWITCH_HANDOFF.md").write_text(
                "# legacy\n", encoding="utf-8"
            )

            report = SCAN_HANDOFFS.scan_handoffs(root)

            self.assertEqual(report["handoffs"][0]["status"], "invalid")
            self.assertEqual(report["counts"]["invalid"], 1)

    def test_format_table_contains_status_and_feedback_state(self) -> None:
        report = {
            "handoffs": [
                {
                    "handoff_id": "demo",
                    "title": "演示",
                    "status": "in_progress",
                    "sender_agent": "codex",
                    "executor_agent": "doubao",
                    "created_at": "2026-08-31",
                    "package_dir": "/tmp/demo",
                    "feedback_exists": False,
                }
            ],
            "counts": {"total": 1, "in_progress": 1},
        }

        table = SCAN_HANDOFFS.format_table(report)

        self.assertIn("demo", table)
        self.assertIn("in_progress", table)
        self.assertIn("待反馈", table)


if __name__ == "__main__":
    unittest.main()
