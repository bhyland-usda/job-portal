#!/usr/bin/env python3
import argparse
import queue
import re
import shutil
import subprocess
import sys
import threading
import time
import unittest
from pathlib import Path


def format_duration(seconds: float | None) -> str:
    if seconds is None:
        return "--:--"
    total = max(0, int(seconds))
    mins, secs = divmod(total, 60)
    hours, mins = divmod(mins, 60)
    if hours:
        return f"{hours:02d}:{mins:02d}:{secs:02d}"
    return f"{mins:02d}:{secs:02d}"


def iter_cases(suite: unittest.TestSuite):
    for test in suite:
        if isinstance(test, unittest.TestSuite):
            yield from iter_cases(test)
        else:
            yield test


def discover_total_tests(repo_root: Path, k_filters: list[str]) -> int:
    tests_dir = repo_root / "tests"
    if not tests_dir.exists():
        return 0

    loader = unittest.defaultTestLoader
    suite = loader.discover(start_dir=str(tests_dir), pattern="test_*.py")
    cases = list(iter_cases(suite))

    if not k_filters:
        return len(cases)

    def match(case_id: str) -> bool:
        return any(k in case_id for k in k_filters)

    return sum(1 for case in cases if match(case.id()))


def render_bar(completed: int, total: int, width: int = 28) -> str:
    if total <= 0:
        return "[" + ("=" * (width // 2)).ljust(width, "-") + "]"
    ratio = min(1.0, max(0.0, completed / total))
    filled = int(width * ratio)
    return "[" + ("#" * filled) + ("-" * (width - filled)) + "]"


def main() -> int:
    parser = argparse.ArgumentParser(description="Run command with unittest-style progress bar")
    parser.add_argument("--repo-root", default=".", help="Repository root for test discovery")
    parser.add_argument("--", dest="cmd_sep", action="store_true")
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()

    command = args.command
    if command and command[0] == "--":
        command = command[1:]
    if not command:
        print("No command provided.", file=sys.stderr)
        return 2

    repo_root = Path(args.repo_root).resolve()

    # Parse -k filters from command for ETA denominator.
    k_filters: list[str] = []
    for idx, token in enumerate(command):
        if token == "-k" and idx + 1 < len(command):
            k_filters.append(command[idx + 1])

    try:
        total = discover_total_tests(repo_root, k_filters)
    except Exception:
        total = 0

    proc = subprocess.Popen(
        command,
        cwd=str(repo_root),
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,
    )

    q: queue.Queue[str | None] = queue.Queue()

    def reader_thread():
        assert proc.stdout is not None
        for line in proc.stdout:
            q.put(line)
        q.put(None)

    t = threading.Thread(target=reader_thread, daemon=True)
    t.start()

    status = "starting"
    interaction_status = ""
    completed = 0
    passed = 0
    failed = 0
    errored = 0
    skipped = 0
    start = time.time()

    result_line = re.compile(
        r"^(?P<name>\S+)\s+\((?P<case>[^)]+)\)\s+\.\.\.\s+(?P<result>ok|FAIL|ERROR|skipped.*)$"
    )
    progress_line = re.compile(r"^PROGRESS\s+done=(?P<done>\d+)\s+total=(?P<total>\d+)\s+status=(?P<status>.*)$")
    progress_result_line = re.compile(r"^PROGRESS_RESULT\s+outcome=(?P<outcome>pass|fail|error|skip)\s+status=(?P<status>.*)$")
    interaction_mode = False
    local_interaction_done = 0
    local_interaction_total = 0
    base_interaction_done = 0
    base_interaction_total = 0
    segment_index = 0
    segment_locked_total: int | None = None
    announced_first_lock = False
    interaction_passed = 0
    interaction_failed = 0
    interaction_errored = 0
    interaction_skipped = 0

    end_of_stream = False
    pending_tail: list[str] = []
    last_render_len = 0

    while True:
        try:
            item = q.get(timeout=0.2)
        except queue.Empty:
            item = ""

        if item is None:
            end_of_stream = True
        elif item:
            line = item.rstrip("\n")
            p = progress_line.match(line.strip())
            if p:
                interaction_mode = True
                next_done = int(p.group("done"))
                next_total = int(p.group("total"))

                # A lower done/total indicates a new test method started; carry completed segment into base totals.
                if next_done < local_interaction_done or next_total < local_interaction_total:
                    base_interaction_done += local_interaction_done
                    base_interaction_total += local_interaction_total
                    segment_index += 1
                    local_interaction_done = 0
                    local_interaction_total = 0
                    segment_locked_total = None

                if segment_locked_total is None and next_total > 0:
                    segment_locked_total = next_total
                    sys.stdout.write("\n")
                    sys.stdout.write(f"[progress] segment {segment_index + 1} locked total={segment_locked_total}\n")
                    if not announced_first_lock:
                        sys.stdout.write(f"[progress] interaction plan lock acquired at total={segment_locked_total}\n")
                        announced_first_lock = True
                    sys.stdout.flush()

                # Keep denominator stable within a segment once it is locked.
                if segment_locked_total is not None:
                    if next_total != segment_locked_total and next_done >= local_interaction_done:
                        interaction_status = (
                            f"{p.group('status').strip()} | total-mutation {next_total}->{segment_locked_total} ignored"
                        )
                    else:
                        interaction_status = p.group("status").strip() or interaction_status
                    local_interaction_total = segment_locked_total
                else:
                    local_interaction_total = max(local_interaction_total, next_total)
                    interaction_status = p.group("status").strip() or interaction_status

                local_interaction_done = next_done
                continue
            r = progress_result_line.match(line.strip())
            if r:
                outcome = r.group('outcome')
                if outcome == 'pass':
                    interaction_passed += 1
                elif outcome == 'fail':
                    interaction_failed += 1
                elif outcome == 'error':
                    interaction_errored += 1
                elif outcome == 'skip':
                    interaction_skipped += 1
                continue
            m = result_line.match(line.strip())
            if m:
                completed += 1
                case_name = m.group("case")
                result = m.group("result")
                low = result.lower()
                if low.startswith("ok"):
                    passed += 1
                elif low.startswith("fail"):
                    failed += 1
                elif low.startswith("error"):
                    errored += 1
                elif low.startswith("skipped"):
                    skipped += 1
                status = f"{case_name} -> {result}"
            else:
                pending_tail.append(line)
                if len(pending_tail) > 20:
                    pending_tail = pending_tail[-20:]

        elapsed = time.time() - start
        if interaction_mode and (base_interaction_total + local_interaction_total) > 0:
            view_done = base_interaction_done + local_interaction_done
            per_case_interaction_mode = (segment_locked_total == 1 and total > 0)
            if per_case_interaction_mode:
                # One unittest case equals one interaction; keep denominator fixed to discovered tests.
                outcomes_done = interaction_passed + interaction_failed + interaction_errored + interaction_skipped
                view_done = max(view_done, outcomes_done)
                view_total = max(total, view_done)
            else:
                view_total = base_interaction_total + local_interaction_total
            view_status = interaction_status or status
            view_passed = interaction_passed
            view_failed = interaction_failed
            view_errored = interaction_errored
            view_skipped = interaction_skipped
        else:
            view_done = completed
            view_total = total
            view_status = status
            view_passed = passed
            view_failed = failed
            view_errored = errored
            view_skipped = skipped

        eta = None
        if view_total > 0 and view_done > 0:
            eta = (elapsed / view_done) * max(0, view_total - view_done)

        pct = 0.0 if view_total <= 0 else (view_done / view_total) * 100.0
        bar = render_bar(view_done, view_total)
        summary = (
            f"\r{bar} {view_done}/{view_total if view_total > 0 else '?'} "
            f"{pct:5.1f}% | elapsed {format_duration(elapsed)} | "
            f"eta {format_duration(eta)} | pass {view_passed} fail {view_failed} error {view_errored} skip {view_skipped} | {view_status[:72]}"
        )
        cols = shutil.get_terminal_size(fallback=(120, 20)).columns
        visible = summary[1:]
        if len(visible) > cols:
            visible = visible[: max(0, cols - 1)]
        pad = " " * max(0, last_render_len - len(visible))
        sys.stdout.write("\r" + visible + pad)
        sys.stdout.flush()
        last_render_len = len(visible)

        if end_of_stream:
            break

    return_code = proc.wait()
    sys.stdout.write("\n")

    # Print final tail output from unittest for context.
    for line in pending_tail[-8:]:
        if line.strip():
            print(line)

    return return_code


if __name__ == "__main__":
    raise SystemExit(main())
