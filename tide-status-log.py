#!/usr/bin/env python3
"""Inspect status-controller writes, watermarks, and completed sync statistics."""

import argparse
from collections import Counter
from datetime import datetime
import json
import re
import sys
import time


WRITE = re.compile(
    r"^CreateStatus\(([^,]+), ([^,]+), ([^,]+), "
    r"\{(\S+) (\S*) (.*) tide\}\)$", re.DOTALL
)
QUALIFIER = re.compile(r'(?<!\S)(?:org|repo):(?:"([^"]+)"|(\S+))')
SHRINK = "Search page exceeded time or resource limits, retrying with a smaller page size."
QUERY_EVENTS = {"Sending query", "Finished query", "Searched for open PRs.", SHRINK}


def query_orgs(query):
    return {((quoted or bare).split("/", 1)[0])
            for quoted, bare in QUALIFIER.findall(query)}


def clean(value):
    return " ".join(str(value).split())


def error_kinds(error):
    kinds = set()
    if "secondary rate limit" in error.lower():
        kinds.add("secondary_rate_limit")
    if "Resource limits for this query exceeded" in error:
        kinds.add("resource_limit")
    kinds.update("http_" + code for code in re.findall(r"status code: (\d{3})", error))
    return kinds or {"other"}


def watermark_detail(watermark, observed):
    try:
        age = (datetime.fromisoformat(observed.replace("Z", "+00:00"))
               - datetime.fromisoformat(watermark.replace("Z", "+00:00")))
        return f"age at observation: {int(age.total_seconds())}s"
    except (ValueError, TypeError):
        return ""


def lines(stream, follow):
    # Hold an incomplete final line until its remaining bytes arrive.
    pending = ""
    while True:
        chunk = stream.readline()
        if chunk:
            pending += chunk
            if pending.endswith("\n"):
                yield pending
                pending = ""
        elif follow:
            time.sleep(0.5)
        else:
            if pending:
                yield pending
            return


def main():
    parser = argparse.ArgumentParser(
        description=__doc__,
        epilog=("WRITE is an API attempt, not confirmation of success. FAILED "
                "reports a logged failure; Tide silently ignores some 404s. "
                "Watermarks describe PR updated_at search progress, not status "
                "write completion. SYNC and QUERY rows summarize each completed "
                "loop. PR counts are search results, excluding merge-pool fallback; "
                "with --repo, query counts still cover the entire org. Calls count "
                "logged search calls, excluding retries inside the GitHub client. "
                "Statistics need debug logs; the first cycle may be truncated. "
                "Follow mode watches appends to the same file.")
    )
    parser.add_argument("log", help="JSONL log file, or - for stdin")
    parser.add_argument("--org", help="Only this GitHub organization")
    parser.add_argument("--repo", help="Only this org/repo; watermarks stay at org scope")
    parser.add_argument("-f", "--follow", action="store_true", help="Follow appended log lines")
    parser.add_argument("--watermarks-only", action="store_true",
                        help="Hide write/failure rows; retain watermarks and cycle summaries")
    args = parser.parse_args()
    if args.repo and (len(args.repo.split("/")) != 2 or not all(args.repo.split("/"))):
        parser.error("--repo must be org/repo")
    if args.repo and args.org and args.repo.split("/")[0] != args.org:
        parser.error("--org and --repo must refer to the same organization")
    wanted_org = args.org or (args.repo.split("/")[0] if args.repo else None)
    writes, failures = Counter(), Counter()
    watermarks = {}
    malformed = 0
    queries = {}
    cycle_counts = Counter()
    cycle_errors = set()
    first_cycle = True

    def selected(org, repo=None):
        return ((not wanted_org or org == wanted_org)
                and (repo is None or not args.repo or f"{org}/{repo}" == args.repo))

    def emit(at, event, target, sha, value, detail):
        print("\t".join(map(clean, (at, event, target, sha, value, detail))), flush=True)

    stream = sys.stdin if args.log == "-" else open(args.log, encoding="utf-8")
    print("TIME\tEVENT\tTARGET\tSHA\tSTATE_OR_WATERMARK\tDETAILS", flush=True)
    try:
        for line in lines(stream, args.follow and args.log != "-"):
            try:
                row = json.loads(line)
            except ValueError:
                malformed += 1
                continue
            if not isinstance(row, dict) or row.get("controller") != "status-update":
                continue
            msg, at = row.get("msg", ""), row.get("time", "")
            query = row.get("query", "")
            if msg in QUERY_EVENTS and query:
                orgs = query_orgs(query)
                if not wanted_org or wanted_org in orgs:
                    stats = queries.setdefault(query, {
                        "target": ",".join(sorted(orgs)) or "unknown-query",
                        "result": "unknown", "prs": 0, "calls": 0,
                        "shrink_to": [], "shrink_errors": set(),
                    })
                    if msg == "Sending query":
                        stats["calls"] += 1
                    elif msg == SHRINK:
                        stats["shrink_to"].append(row.get("search_page_size", "?"))
                        stats["shrink_errors"].update(error_kinds(row.get("error", "")))
                        stats["page_size"] = row.get("search_page_size", "?")
                    elif msg == "Finished query":
                        stats.update(result="success", prs=row.get("pr_found_count", 0),
                                     duration=row.get("duration", "?"),
                                     page_size=row.get("search_page_size", "?"),
                                     cost=row.get("cost", "?"),
                                     remaining=row.get("remaining", "?"))
                    elif msg == "Searched for open PRs.":
                        stats["prs"] = row.get("result_count", 0)
                        stats["duration"] = row.get("duration", "?")
                        if stats["result"] != "success":
                            stats["result"] = "partial" if stats["prs"] else "error"
            if msg in ("Search partially completed", "Search failed"):
                cycle_errors.update(error_kinds(row.get("error", "")))
            if msg == "Statuses synced.":
                outcomes = Counter(s["result"] for s in queries.values())
                prs = sum(s["prs"] for s in queries.values())
                shrinks = sum(len(s["shrink_to"]) for s in queries.values())
                details = (f"duration={row.get('duration', '?')} queries={len(queries)} "
                           f"outcomes={','.join(f'{k}:{v}' for k, v in sorted(outcomes.items())) or 'unknown'} "
                           f"calls={sum(s['calls'] for s in queries.values())} shrinks={shrinks} "
                           f"write_attempts={cycle_counts['writes']} logged_failures={cycle_counts['failures']}")
                if args.repo:
                    details += f" writes_scope={args.repo} search_scope={wanted_org}"
                if cycle_errors:
                    # The aggregate error has no org label, so don't attribute it to a filtered org.
                    details += " whole_cycle_errors=" + ",".join(sorted(cycle_errors))
                if first_cycle:
                    details += " coverage=first-cycle-may-be-truncated"
                emit(at, "SYNC", wanted_org or "all-orgs", "-", f"prs_returned={prs}", details)
                for stats in sorted(queries.values(), key=lambda s: s["target"]):
                    reductions = "->".join(map(str, stats["shrink_to"])) or "none-observed"
                    detail = (f"prs={stats['prs']} duration={stats.get('duration', '?')} "
                              f"calls={stats['calls']} shrink_to={reductions} "
                              f"page_size={stats.get('page_size', '?')}")
                    if stats["shrink_errors"]:
                        detail += " shrink_errors=" + ",".join(sorted(stats["shrink_errors"]))
                    if "cost" in stats:
                        detail += f" cost={stats['cost']} remaining={stats['remaining']}"
                    if stats["target"] in watermarks:
                        detail += " watermark=" + watermarks[stats["target"]][0]
                    emit(at, "QUERY", stats["target"], "-", stats["result"], detail)
                queries.clear()
                cycle_counts.clear()
                cycle_errors.clear()
                first_cycle = False
            match = WRITE.fullmatch(msg)
            if match:
                org, repo, sha, state, _, description = match.groups()
                if selected(org, repo):
                    writes[f"{org}/{repo}"] += 1
                    cycle_counts["writes"] += 1
                    if not args.watermarks_only:
                        emit(at, "WRITE", f"{org}/{repo}", sha, state, description)
            elif msg.startswith("Failed to set status context"):
                org, repo = row.get("org", ""), row.get("repo", "")
                if selected(org, repo):
                    failures[f"{org}/{repo}"] += 1
                    cycle_counts["failures"] += 1
                    if not args.watermarks_only:
                        emit(at, "FAILED", f"{org}/{repo}", row.get("sha", ""), "-",
                             f"PR #{row.get('pr', '?')}: {row.get('error', msg)}")
            elif msg in ("Advanced start time", "no new results") and row.get("latestPR"):
                orgs = query_orgs(row.get("query", ""))
                if wanted_org and wanted_org not in orgs:
                    continue
                target = ",".join(sorted(orgs)) or "unknown-query"
                watermark = row["latestPR"]
                changed = target not in watermarks or watermarks[target][0] != watermark
                watermarks[target] = (watermark, at)
                if changed:
                    emit(at, "WATERMARK", target, "-", watermark,
                         watermark_detail(watermark, at))
    except KeyboardInterrupt:
        pass
    finally:
        if stream is not sys.stdin:
            stream.close()

    print(f"\n# {sum(writes.values())} write attempts; "
          f"{sum(failures.values())} logged failures", file=sys.stderr)
    if malformed:
        print(f"# Skipped {malformed} invalid JSON lines", file=sys.stderr)
    print("# Latest observed search watermarks (PR updated_at minus 30s):", file=sys.stderr)
    for org, (watermark, at) in sorted(watermarks.items()):
        print(f"# {org}: {watermark} (observed {at})", file=sys.stderr)


if __name__ == "__main__":
    try:
        main()
    except BrokenPipeError:
        # Allow piping the output to head without a traceback.
        sys.stdout = open("/dev/null", "w")
