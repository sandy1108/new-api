#!/usr/bin/env python3
"""扫描 New API 工程中的 Agent 交接包并汇总执行状态。

交接包可能包含私有备份内容，因此默认只输出元数据和路径，不读取或打印
交接正文、执行日志和其他敏感文件的内容。
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from pathlib import Path
from typing import Any, Iterable


STATUSES = (
    "pending",
    "in_progress",
    "completed",
    "blocked",
    "failed",
    "rolled_back",
    "cancelled",
)

METADATA_FILENAME = "handoff.json"
FEEDBACK_FILENAME = "EXECUTION_FEEDBACK.md"
HANDOFF_MARKERS = (
    "HANDOFF.md",
    "handoff.md",
    "PRODUCTION_SWITCH_HANDOFF.md",
    "production-image-switch-handoff.md",
)
LEGACY_DIRECTORY_PREFIXES = ("pre-switch-", "release-", "pre-sync-")


def _absolute(path: Path) -> Path:
    """返回规范化绝对路径，便于输出可直接定位的结果。"""

    return path.expanduser().resolve()


def _path_inside(path: Path, directory: Path) -> bool:
    """判断元数据引用是否仍位于交接包目录内，阻止路径穿越。"""

    try:
        path.relative_to(directory)
    except ValueError:
        return False
    return True


def _resolve_package_file(
    package_dir: Path, relative_name: Any, field_name: str, errors: list[str]
) -> Path | None:
    """解析交接包内文件引用，并记录缺失或越界引用。"""

    if not isinstance(relative_name, str) or not relative_name.strip():
        errors.append(f"metadata.{field_name} must be a non-empty relative path")
        return None

    candidate = _absolute(package_dir / relative_name)
    if not _path_inside(candidate, package_dir):
        errors.append(f"metadata.{field_name} escapes package directory")
        return None
    return candidate


def _load_record(
    package_dir: Path,
    *,
    source: str,
    legacy: bool = False,
    marker: Path | None = None,
) -> dict[str, Any]:
    """读取一个交接包；单个包损坏时也返回可供总扫描继续使用的记录。"""

    package_dir = _absolute(package_dir)
    metadata_path = package_dir / METADATA_FILENAME
    metadata_exists = metadata_path.is_file()
    errors: list[str] = []
    metadata: dict[str, Any] = {}

    if metadata_exists:
        try:
            loaded = json.loads(metadata_path.read_text(encoding="utf-8"))
            if not isinstance(loaded, dict):
                errors.append("metadata root must be an object")
            else:
                metadata = loaded
        except (OSError, UnicodeError, json.JSONDecodeError) as exc:
            errors.append(f"cannot read metadata: {exc}")
    elif not legacy:
        errors.append("metadata file is missing")

    if legacy and not metadata and not metadata_exists:
        # 旧式目录没有机器可读元数据，只做兼容登记；后续应补 handoff.json。
        handoff_id = f"legacy-{package_dir.name}"
        title = f"旧式交接包：{package_dir.name}"
        status = "pending"
        sender_agent = "codex"
        executor_agent = "doubao"
        created_at = None
        handoff_name = marker.name if marker else None
        feedback_name = FEEDBACK_FILENAME
        errors.append("metadata file is missing; legacy package inferred as pending")
    else:
        handoff_id = metadata.get("handoff_id")
        title = metadata.get("title")
        status = metadata.get("status")
        sender_agent = metadata.get("sender_agent")
        executor_agent = metadata.get("executor_agent")
        created_at = metadata.get("created_at")
        handoff_name = metadata.get("handoff_file")
        feedback_name = metadata.get("feedback_file")

    required_strings = {
        "handoff_id": handoff_id,
        "title": title,
        "sender_agent": sender_agent,
        "executor_agent": executor_agent,
    }
    for field_name, value in required_strings.items():
        if not isinstance(value, str) or not value.strip():
            errors.append(f"metadata.{field_name} must be a non-empty string")

    if status not in STATUSES:
        errors.append(f"metadata.status must be one of: {', '.join(STATUSES)}")
        status = "invalid"

    if not isinstance(created_at, (str, type(None))):
        errors.append("metadata.created_at must be a string or null")

    handoff_path = _resolve_package_file(package_dir, handoff_name, "handoff_file", errors)
    feedback_path = _resolve_package_file(package_dir, feedback_name, "feedback_file", errors)

    # 旧包没有元数据时，marker 是唯一可靠的交接正文路径。
    if handoff_path is None and marker is not None:
        handoff_path = _absolute(marker)
    if feedback_path is None:
        feedback_path = package_dir / FEEDBACK_FILENAME

    if handoff_path is not None and not handoff_path.is_file():
        errors.append("handoff file is missing")
    if feedback_path is not None and not feedback_path.is_file():
        errors.append("feedback file is missing")

    return {
        "handoff_id": handoff_id if isinstance(handoff_id, str) else f"invalid-{package_dir.name}",
        "title": title if isinstance(title, str) else package_dir.name,
        "status": status,
        "sender_agent": sender_agent if isinstance(sender_agent, str) else None,
        "executor_agent": executor_agent if isinstance(executor_agent, str) else None,
        "created_at": created_at,
        "started_at": metadata.get("started_at"),
        "completed_at": metadata.get("completed_at"),
        "result": metadata.get("result"),
        "source": source,
        "legacy": legacy,
        "package_dir": str(package_dir),
        "metadata_file": str(metadata_path) if metadata_path.is_file() else None,
        "handoff_file": str(handoff_path) if handoff_path is not None else None,
        "feedback_file": str(feedback_path) if feedback_path is not None else None,
        "handoff_exists": bool(handoff_path and handoff_path.is_file()),
        "feedback_exists": bool(feedback_path and feedback_path.is_file()),
        "errors": errors,
    }


def _metadata_directories(root: Path) -> Iterable[Path]:
    """遍历标准目录中的交接包目录，排除模板目录本身。

    即使 `handoff.json` 遗漏，也要把带有任务说明的目录报告为 invalid，
    这样扫描结果不会把“已生成但不完整”的交接包静默吞掉。
    """

    if not root.is_dir():
        return ()
    package_dirs: set[Path] = set()
    for candidate in root.rglob("*"):
        if not candidate.is_dir():
            continue
        if "templates" in candidate.relative_to(root).parts:
            continue
        if (candidate / METADATA_FILENAME).is_file() or any(
            (candidate / marker_name).is_file() for marker_name in HANDOFF_MARKERS
        ):
            package_dirs.add(candidate)
    return sorted(package_dirs)


def _legacy_directories(backup_root: Path) -> Iterable[tuple[Path, Path | None]]:
    """发现旧式备份目录；仅扫描备份根目录的直接子目录。"""

    if not backup_root.is_dir():
        return ()

    discovered: list[tuple[Path, Path | None]] = []
    for child in sorted(backup_root.iterdir()):
        if not child.is_dir() or not child.name.startswith(LEGACY_DIRECTORY_PREFIXES):
            continue
        marker = next(
            (child / marker_name for marker_name in HANDOFF_MARKERS if (child / marker_name).is_file()),
            None,
        )
        if marker is not None or (child / METADATA_FILENAME).is_file():
            discovered.append((child, marker))
    return discovered


def _discover_directories(project_root: Path, backup_root: Path) -> list[tuple[Path, str, bool, Path | None]]:
    """收集标准项目目录、标准备份目录和旧式备份目录，并去重。"""

    candidates: list[tuple[Path, str, bool, Path | None]] = []
    seen: set[Path] = set()
    sources = (
        (project_root / "docs/development/handoffs", "project"),
        (backup_root / "handoffs", "backup"),
    )
    for root, source in sources:
        for package_dir in _metadata_directories(root):
            package_dir = _absolute(package_dir)
            if package_dir not in seen:
                candidates.append((package_dir, source, False, None))
                seen.add(package_dir)

    for package_dir, marker in _legacy_directories(backup_root):
        package_dir = _absolute(package_dir)
        if package_dir not in seen:
            candidates.append((package_dir, "backup", True, marker))
            seen.add(package_dir)
    return candidates


def scan_handoffs(project_root: Path, backup_root: Path | None = None) -> dict[str, Any]:
    """扫描交接包并返回适合表格或 JSON 输出的报告。"""

    project_root = _absolute(project_root)
    backup_root = _absolute(backup_root or project_root.parent / ".backups/new-api")
    records = [
        _load_record(package_dir, source=source, legacy=legacy, marker=marker)
        for package_dir, source, legacy, marker in _discover_directories(project_root, backup_root)
    ]
    records.sort(key=lambda item: (item["created_at"] or "", item["handoff_id"]))

    counts = Counter(item["status"] for item in records)
    count_report: dict[str, int] = {"total": len(records)}
    count_report.update({status: counts.get(status, 0) for status in STATUSES})
    count_report["invalid"] = counts.get("invalid", 0)

    warnings: list[str] = []
    by_id: dict[str, list[str]] = {}
    for item in records:
        by_id.setdefault(item["handoff_id"], []).append(item["package_dir"])
    for handoff_id, package_dirs in by_id.items():
        if len(package_dirs) > 1:
            warnings.append(
                f"duplicate handoff_id {handoff_id}: {', '.join(sorted(package_dirs))}"
            )

    return {
        "project_root": str(project_root),
        "backup_root": str(backup_root),
        "handoffs": records,
        "counts": count_report,
        "warnings": warnings,
    }


def format_table(report: dict[str, Any]) -> str:
    """将报告格式化为适合终端阅读的简洁表格。"""

    headers = ("交接包 ID", "状态", "标题", "发包→执行", "创建日期", "反馈", "目录")
    rows = [headers]
    for item in report["handoffs"]:
        feedback = "已填写" if item["feedback_exists"] else "待反馈"
        rows.append(
            (
                item["handoff_id"],
                item["status"],
                item["title"],
                f"{item['sender_agent'] or '-'}→{item['executor_agent'] or '-'}",
                item["created_at"] or "-",
                feedback,
                item["package_dir"],
            )
        )

    widths = [max(len(str(row[index])) for row in rows) for index in range(len(headers))]
    lines = [
        " | ".join(str(value).ljust(widths[index]) for index, value in enumerate(row))
        for row in rows
    ]
    lines.append("")
    count_text = ", ".join(
        f"{status}={report['counts'].get(status, 0)}"
        for status in (
            "pending",
            "in_progress",
            "completed",
            "blocked",
            "failed",
            "rolled_back",
            "cancelled",
            "invalid",
        )
    )
    lines.append(f"总数={report['counts']['total']}；{count_text}")
    if report.get("warnings"):
        lines.append("警告：")
        lines.extend(f"- {warning}" for warning in report["warnings"])
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--project-root",
        type=Path,
        default=Path(__file__).resolve().parents[1],
        help="New API Worktree 根目录（默认是脚本所在 Worktree）",
    )
    parser.add_argument(
        "--backup-root",
        type=Path,
        help="交接包备份根目录（默认是项目根目录同级的 .backups/new-api）",
    )
    parser.add_argument(
        "--format",
        choices=("table", "json"),
        default="table",
        help="输出格式",
    )
    args = parser.parse_args(argv)
    project_root = _absolute(args.project_root)
    if not project_root.is_dir():
        print(f"项目根目录不存在：{project_root}", file=sys.stderr)
        return 2

    report = scan_handoffs(project_root, args.backup_root)
    if args.format == "json":
        print(json.dumps(report, ensure_ascii=False, indent=2))
    else:
        print(format_table(report))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
