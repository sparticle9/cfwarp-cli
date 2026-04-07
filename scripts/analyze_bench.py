#!/usr/bin/env python3
from __future__ import annotations

import argparse
import csv
import json
import math
import statistics
from collections import defaultdict
from pathlib import Path
from typing import Any


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Analyze cfwarp benchmark CSVs and generate markdown + JSON summaries.")
    p.add_argument("--results-dir", default="bench-results")
    p.add_argument("--timestamp", required=True)
    p.add_argument("--proxy-host", default="unknown")
    p.add_argument("--bench-server", default="unknown")
    p.add_argument("--alpine-image", default="unknown")
    p.add_argument("--debian-image", default="unknown")
    p.add_argument("--microwarp-image", default="unknown")
    p.add_argument("--container-cpus", default="unknown")
    p.add_argument("--container-memory", default="unknown")
    p.add_argument("--iperf3-duration", default="unknown")
    p.add_argument("--iperf3-parallel", default="unknown")
    p.add_argument("--latency-runs", default="unknown")
    p.add_argument("--throughput-runs", default="unknown")
    p.add_argument("--http-file-mb", default="unknown")
    p.add_argument("--output")
    p.add_argument("--json-output")
    return p.parse_args()


def load_csv(path: Path) -> list[dict[str, str]]:
    with path.open(newline="") as f:
        return list(csv.DictReader(f))


def to_float(value: str | None) -> float | None:
    if value is None:
        return None
    s = value.strip()
    if not s or s.upper() == "ERR":
        return None
    try:
        return float(s)
    except ValueError:
        return None


def parse_percent(value: str | None) -> float | None:
    if value is None:
        return None
    return to_float(value.replace("%", ""))


def parse_size_to_bytes(value: str | None) -> float | None:
    if value is None:
        return None
    s = value.strip()
    if not s or s.upper() == "ERR":
        return None
    units = {
        "B": 1,
        "kB": 1000,
        "MB": 1000**2,
        "GB": 1000**3,
        "TB": 1000**4,
        "KiB": 1024,
        "MiB": 1024**2,
        "GiB": 1024**3,
        "TiB": 1024**4,
    }
    for unit in sorted(units, key=len, reverse=True):
        if s.endswith(unit):
            n = to_float(s[:-len(unit)])
            return None if n is None else n * units[unit]
    return to_float(s)


def percentile(values: list[float], p: float) -> float:
    if not values:
        return float("nan")
    xs = sorted(values)
    if len(xs) == 1:
        return xs[0]
    rank = (len(xs) - 1) * p
    lo = math.floor(rank)
    hi = math.ceil(rank)
    if lo == hi:
        return xs[lo]
    return xs[lo] + (xs[hi] - xs[lo]) * (rank - lo)


def mean(values: list[float]) -> float:
    return statistics.fmean(values) if values else float("nan")


def stddev(values: list[float]) -> float:
    return statistics.pstdev(values) if len(values) > 1 else 0.0


def safe_fmt(value: float | None, digits: int = 1, scale: float = 1.0) -> str:
    if value is None or math.isnan(value):
        return "n/a"
    return f"{value / scale:.{digits}f}"


def human_bytes(num: float | None) -> str:
    if num is None or math.isnan(num):
        return "n/a"
    value = float(num)
    for unit in ["B", "KiB", "MiB", "GiB", "TiB"]:
        if abs(value) < 1024.0 or unit == "TiB":
            return f"{value:.2f} {unit}"
        value /= 1024.0
    return f"{value:.2f} TiB"


def common_variants(*sections: dict[str, Any]) -> list[str]:
    keys = None
    for section in sections:
        present = set(section)
        keys = present if keys is None else keys & present
    return sorted(keys or [])


def collect_latency(rows: list[dict[str, str]]) -> dict[str, dict[str, Any]]:
    grouped: dict[str, dict[str, Any]] = defaultdict(lambda: {"success": [], "errors": 0})
    for row in rows:
        variant = row.get("variant", "unknown")
        rtt = to_float(row.get("rtt_ms"))
        if rtt is None:
            grouped[variant]["errors"] += 1
        else:
            grouped[variant]["success"].append(rtt)
    return grouped


def collect_iperf3(rows: list[dict[str, str]]) -> dict[str, dict[str, Any]]:
    grouped: dict[str, dict[str, Any]] = defaultdict(
        lambda: {
            "upload": [],
            "download": [],
            "upload_retransmits": [],
            "download_retransmits": [],
            "errors": 0,
            "runs": defaultdict(set),
        }
    )
    for row in rows:
        variant = row.get("variant", "unknown")
        direction = (row.get("direction") or "").strip()
        run = (row.get("run") or "").strip()
        bps = to_float(row.get("bits_per_second"))
        retransmits = to_float(row.get("retransmits")) or 0.0
        if direction not in ("upload", "download") or bps is None or bps <= 0:
            grouped[variant]["errors"] += 1
            continue
        grouped[variant][direction].append(bps)
        grouped[variant][f"{direction}_retransmits"].append(retransmits)
        if run:
            grouped[variant]["runs"][run].add(direction)
    return grouped


def collect_http(rows: list[dict[str, str]]) -> dict[str, dict[str, Any]]:
    grouped: dict[str, dict[str, Any]] = defaultdict(lambda: {"speed_bps": [], "time_total_s": [], "errors": 0, "codes": []})
    for row in rows:
        variant = row.get("variant", "unknown")
        speed = to_float(row.get("speed_bps"))
        total = to_float(row.get("time_total_s"))
        code = (row.get("http_code") or "").strip()
        grouped[variant]["codes"].append(code)
        if speed is None or speed <= 0 or code != "200":
            grouped[variant]["errors"] += 1
        else:
            grouped[variant]["speed_bps"].append(speed)
            if total is not None:
                grouped[variant]["time_total_s"].append(total)
    return grouped


def collect_k6(rows: list[dict[str, str]]) -> dict[str, dict[str, list[dict[str, Any]]]]:
    grouped: dict[str, dict[str, list[dict[str, Any]]]] = defaultdict(lambda: defaultdict(list))
    for row in rows:
        grouped[row.get("variant", "unknown")][row.get("profile", "unknown")].append(
            {
                "vus": to_float(row.get("vus")),
                "duration_s": to_float(row.get("duration_s")),
                "http_reqs": to_float(row.get("http_reqs")),
                "rps": to_float(row.get("rps")),
                "error_rate": to_float(row.get("error_rate")),
                "p50_ms": to_float(row.get("p50_ms")),
                "p95_ms": to_float(row.get("p95_ms")),
                "p99_ms": to_float(row.get("p99_ms")),
                "max_ms": to_float(row.get("max_ms")),
                "waiting_p95_ms": to_float(row.get("waiting_p95_ms")),
                "connect_p95_ms": to_float(row.get("connect_p95_ms")),
                "tls_p95_ms": to_float(row.get("tls_p95_ms")),
                "blocked_p95_ms": to_float(row.get("blocked_p95_ms")),
                "iteration_p95_ms": to_float(row.get("iteration_p95_ms")),
            }
        )
    return grouped


def collect_stats(rows: list[dict[str, str]]) -> dict[tuple[str, str], dict[str, Any]]:
    grouped: dict[tuple[str, str], dict[str, Any]] = defaultdict(
        lambda: {
            "samples": [],
            "cpu": [],
            "mem_used": [],
            "mem_limit": [],
            "net_rx": [],
            "net_tx": [],
            "timestamps": [],
        }
    )
    for row in rows:
        variant = row.get("variant", "unknown")
        phase = row.get("phase", "unknown")
        entry = grouped[(variant, phase)]
        sample_ts = to_float(row.get("sample_ts"))
        cpu = parse_percent(row.get("cpu_pct"))
        mem_used = parse_size_to_bytes(row.get("mem_used"))
        mem_limit = parse_size_to_bytes(row.get("mem_limit"))
        net_rx = parse_size_to_bytes(row.get("net_rx"))
        net_tx = parse_size_to_bytes(row.get("net_tx"))
        entry["samples"].append(
            {
                "sample_ts": sample_ts,
                "cpu_pct": cpu,
                "mem_used_bytes": mem_used,
                "mem_limit_bytes": mem_limit,
                "net_rx_bytes": net_rx,
                "net_tx_bytes": net_tx,
            }
        )
        if cpu is not None:
            entry["cpu"].append(cpu)
        if mem_used is not None:
            entry["mem_used"].append(mem_used)
        if mem_limit is not None:
            entry["mem_limit"].append(mem_limit)
        if net_rx is not None:
            entry["net_rx"].append(net_rx)
        if net_tx is not None:
            entry["net_tx"].append(net_tx)
        if sample_ts is not None:
            entry["timestamps"].append(sample_ts)
    return grouped


def summarize_stats(grouped: dict[tuple[str, str], dict[str, Any]]) -> tuple[dict[tuple[str, str], dict[str, Any]], dict[tuple[str, str], list[dict[str, Any]]]]:
    summary: dict[tuple[str, str], dict[str, Any]] = {}
    raw: dict[tuple[str, str], list[dict[str, Any]]] = {}
    for key, entry in grouped.items():
        cpu = entry["cpu"]
        mem = entry["mem_used"]
        mem_limit = entry["mem_limit"]
        rx = entry["net_rx"]
        tx = entry["net_tx"]
        summary[key] = {
            "samples": len(entry["samples"]),
            "cpu_mean": mean(cpu) if cpu else float("nan"),
            "cpu_p95": percentile(cpu, 0.95) if cpu else float("nan"),
            "cpu_max": max(cpu) if cpu else float("nan"),
            "mem_mean_bytes": mean(mem) if mem else float("nan"),
            "mem_max_bytes": max(mem) if mem else float("nan"),
            "mem_limit_bytes": max(mem_limit) if mem_limit else float("nan"),
            "net_rx_delta_bytes": (rx[-1] - rx[0]) if len(rx) >= 2 else float("nan"),
            "net_tx_delta_bytes": (tx[-1] - tx[0]) if len(tx) >= 2 else float("nan"),
            "mode": "timeseries" if entry["timestamps"] else "snapshot",
        }
        raw[key] = entry["samples"]
    return summary, raw


def build_markdown(args: argparse.Namespace, latency: dict[str, Any], iperf3: dict[str, Any], http: dict[str, Any], k6: dict[str, Any], stats: dict[tuple[str, str], Any]) -> str:
    lines: list[str] = []
    lines.append("# cfwarp-cli Benchmark Report")
    lines.append("")
    lines.append("| Key | Value |")
    lines.append("|-----|-------|")
    lines.append(f"| Timestamp | {args.timestamp} |")
    lines.append(f"| Proxy host | {args.proxy_host} |")
    lines.append(f"| Bench server | {args.bench_server} |")
    lines.append(f"| Alpine image | {args.alpine_image} |")
    lines.append(f"| Debian image | {args.debian_image} |")
    if args.microwarp_image != "unknown":
        lines.append(f"| MicroWARP image | {args.microwarp_image} |")
    lines.append(f"| Container CPU | {args.container_cpus} vCPU |")
    lines.append(f"| Container memory | {args.container_memory} |")
    lines.append(f"| iperf3 | bidirectional, {args.iperf3_parallel} streams × {args.iperf3_duration}s |")
    lines.append(f"| Latency probes | {args.latency_runs} pings per variant |")
    lines.append(f"| Throughput probes | {args.throughput_runs} runs per variant |")
    lines.append(f"| HTTP file size | {args.http_file_mb} MB |")
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## Latency — ping RTT through WireGuard tunnel (ms)")
    lines.append("")
    lines.append("| Variant | p50 | p95 | p99 | Mean | StdDev | Success | Errors |")
    lines.append("|---------|----:|----:|----:|-----:|-------:|--------:|-------:|")
    for variant in sorted(latency):
        vals = latency[variant]["success"]
        errs = latency[variant]["errors"]
        lines.append(
            f"| {variant:<8} | {safe_fmt(percentile(vals, 0.50) if vals else float('nan'))} | "
            f"{safe_fmt(percentile(vals, 0.95) if vals else float('nan'))} | "
            f"{safe_fmt(percentile(vals, 0.99) if vals else float('nan'))} | "
            f"{safe_fmt(mean(vals) if vals else float('nan'))} | "
            f"{safe_fmt(stddev(vals) if vals else float('nan'))} | {len(vals)} | {errs} |"
        )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## Throughput — iperf3 bidirectional raw WireGuard (Mbit/s)")
    lines.append("")
    lines.append("| Variant | Upload Median | Download Median | Upload p95 | Download p95 | Upload Mean | Download Mean | Upload Retr Mean | Complete Runs | Errors |")
    lines.append("|---------|--------------:|----------------:|-----------:|-------------:|------------:|--------------:|------------------:|--------------:|-------:|")
    for variant in sorted(iperf3):
        up = iperf3[variant]["upload"]
        down = iperf3[variant]["download"]
        errs = iperf3[variant]["errors"]
        complete_runs = sum(1 for dirs in iperf3[variant]["runs"].values() if {"upload", "download"}.issubset(dirs))
        lines.append(
            f"| {variant:<8} | {safe_fmt(percentile(up, 0.50) if up else float('nan'), scale=1e6)} | "
            f"{safe_fmt(percentile(down, 0.50) if down else float('nan'), scale=1e6)} | "
            f"{safe_fmt(percentile(up, 0.95) if up else float('nan'), scale=1e6)} | "
            f"{safe_fmt(percentile(down, 0.95) if down else float('nan'), scale=1e6)} | "
            f"{safe_fmt(mean(up) if up else float('nan'), scale=1e6)} | "
            f"{safe_fmt(mean(down) if down else float('nan'), scale=1e6)} | "
            f"{safe_fmt(mean(iperf3[variant]['upload_retransmits']) if iperf3[variant]['upload_retransmits'] else float('nan'), digits=1)} | "
            f"{complete_runs} | {errs} |"
        )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## Throughput — HTTP download (MB/s)")
    lines.append("")
    lines.append("| Variant | Median | Mean | Median Time (s) | Mean Time (s) | Success | Errors |")
    lines.append("|---------|-------:|-----:|----------------:|--------------:|--------:|-------:|")
    for variant in sorted(http):
        speeds = http[variant]["speed_bps"]
        totals = http[variant]["time_total_s"]
        errs = http[variant]["errors"]
        lines.append(
            f"| {variant:<8} | {safe_fmt(percentile(speeds, 0.50) if speeds else float('nan'), digits=2, scale=1024**2)} | "
            f"{safe_fmt(mean(speeds) if speeds else float('nan'), digits=2, scale=1024**2)} | "
            f"{safe_fmt(percentile(totals, 0.50) if totals else float('nan'), digits=2)} | "
            f"{safe_fmt(mean(totals) if totals else float('nan'), digits=2)} | {len(speeds)} | {errs} |"
        )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## API-like workload — k6 summary")
    lines.append("")
    lines.append("| Variant | Profile | VUs | RPS | Error Rate | p50 ms | p95 ms | p99 ms | Max ms | Waiting p95 | Connect p95 | Iter p95 |")
    lines.append("|---------|---------|---:|----:|-----------:|-------:|-------:|-------:|------:|------------:|------------:|---------:|")
    for variant in sorted(k6):
        for profile in sorted(k6[variant]):
            rows = k6[variant][profile]
            rps = [r['rps'] for r in rows if r['rps'] is not None]
            err = [r['error_rate'] for r in rows if r['error_rate'] is not None]
            p50s = [r['p50_ms'] for r in rows if r['p50_ms'] is not None]
            p95s = [r['p95_ms'] for r in rows if r['p95_ms'] is not None]
            p99s = [r['p99_ms'] for r in rows if r['p99_ms'] is not None]
            maxs = [r['max_ms'] for r in rows if r['max_ms'] is not None]
            waits = [r['waiting_p95_ms'] for r in rows if r['waiting_p95_ms'] is not None]
            conns = [r['connect_p95_ms'] for r in rows if r['connect_p95_ms'] is not None]
            iters = [r['iteration_p95_ms'] for r in rows if r['iteration_p95_ms'] is not None]
            vus = rows[0]['vus'] if rows else None
            lines.append(
                f"| {variant:<8} | {profile:<7} | {safe_fmt(vus, digits=0)} | {safe_fmt(mean(rps))} | {safe_fmt(mean(err), digits=3)} | {safe_fmt(mean(p50s))} | {safe_fmt(mean(p95s))} | {safe_fmt(mean(p99s))} | {safe_fmt(mean(maxs))} | {safe_fmt(mean(waits))} | {safe_fmt(mean(conns))} | {safe_fmt(mean(iters))} |"
            )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## Resource Usage (per benchmarked container over time)")
    lines.append("")
    lines.append("| Variant | Phase | Samples | CPU Mean % | CPU p95 % | CPU Max % | Mem Mean | Mem Peak | Net RX Δ | Net TX Δ |")
    lines.append("|---------|-------|--------:|-----------:|----------:|----------:|---------:|---------:|---------:|---------:|")
    for (variant, phase) in sorted(stats):
        entry = stats[(variant, phase)]
        lines.append(
            f"| {variant:<8} | {phase:<12} | {entry['samples']} | "
            f"{safe_fmt(entry['cpu_mean'], digits=2)} | {safe_fmt(entry['cpu_p95'], digits=2)} | {safe_fmt(entry['cpu_max'], digits=2)} | "
            f"{human_bytes(entry['mem_mean_bytes'])} | {human_bytes(entry['mem_max_bytes'])} | "
            f"{human_bytes(entry['net_rx_delta_bytes'])} | {human_bytes(entry['net_tx_delta_bytes'])} |"
        )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## Efficiency — Throughput vs benchmarked container cost")
    lines.append("")
    lines.append("| Variant | Upload Median (Mbit/s) | Download Median (Mbit/s) | iperf3 CPU Mean % | iperf3 CPU Max % | iperf3 Mem Peak | Upload Mbit/s per CPU% | HTTP Mean (MB/s) | HTTP CPU Mean % | HTTP Mem Peak |")
    lines.append("|---------|----------------------:|------------------------:|------------------:|-----------------:|----------------:|----------------------:|----------------:|----------------:|--------------:|")
    for variant in sorted(set(list(iperf3.keys()) + list(http.keys()))):
        ip_up = iperf3.get(variant, {}).get("upload", [])
        ip_down = iperf3.get(variant, {}).get("download", [])
        http_vals = http.get(variant, {}).get("speed_bps", [])
        ip_stats = stats.get((variant, "iperf3"), {})
        http_stats = stats.get((variant, "http"), {})
        ip_up_median = percentile(ip_up, 0.50) / 1e6 if ip_up else float("nan")
        ip_down_median = percentile(ip_down, 0.50) / 1e6 if ip_down else float("nan")
        ip_cpu_mean = ip_stats.get("cpu_mean", float("nan"))
        ip_cpu_max = ip_stats.get("cpu_max", float("nan"))
        ip_mem_peak = ip_stats.get("mem_max_bytes", float("nan"))
        eff = ip_up_median / ip_cpu_mean if ip_up and ip_cpu_mean and not math.isnan(ip_cpu_mean) and ip_cpu_mean > 0 else float("nan")
        http_mean_mibs = mean(http_vals) / (1024**2) if http_vals else float("nan")
        http_cpu_mean = http_stats.get("cpu_mean", float("nan"))
        http_mem_peak = http_stats.get("mem_max_bytes", float("nan"))
        lines.append(
            f"| {variant:<8} | {safe_fmt(ip_up_median)} | {safe_fmt(ip_down_median)} | {safe_fmt(ip_cpu_mean, digits=2)} | {safe_fmt(ip_cpu_max, digits=2)} | {human_bytes(ip_mem_peak)} | {safe_fmt(eff, digits=2)} | {safe_fmt(http_mean_mibs, digits=2)} | {safe_fmt(http_cpu_mean, digits=2)} | {human_bytes(http_mem_peak)} |"
        )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## Anomalies / failures")
    lines.append("")
    anomalies: list[str] = []
    for variant, section in sorted(latency.items()):
        if section["errors"]:
            anomalies.append(f"- {variant}: latency probe errors={section['errors']}")
    for variant, section in sorted(iperf3.items()):
        if section["errors"]:
            anomalies.append(f"- {variant}: iperf3 failed directions={section['errors']}")
        incomplete = [run for run, dirs in section["runs"].items() if not {"upload", "download"}.issubset(dirs)]
        if incomplete:
            anomalies.append(f"- {variant}: iperf3 incomplete bidirectional runs={','.join(sorted(incomplete))}")
    for variant, section in sorted(http.items()):
        bad_codes = sorted({code for code in section["codes"] if code and code != "200"})
        if section["errors"]:
            suffix = f" (http codes: {', '.join(bad_codes)})" if bad_codes else ""
            anomalies.append(f"- {variant}: HTTP failed runs={section['errors']}{suffix}")
    for variant, profiles in sorted(k6.items()):
        for profile, rows in sorted(profiles.items()):
            if any((r['error_rate'] or 0) >= 0.01 for r in rows):
                anomalies.append(f"- {variant}: k6 profile {profile} exceeded 1% error rate")
    if not anomalies:
        anomalies.append("- No failed runs recorded in the fetched CSVs.")
    lines.extend(anomalies)
    lines.append("")

    shared = common_variants(latency, iperf3, http)
    if shared:
        lines.append("---")
        lines.append("")
        lines.append("## Cross-variant summary")
        lines.append("")
        lines.append("| Variant | Mean latency (ms) | iperf3 upload median (Mbit/s) | iperf3 download median (Mbit/s) | HTTP mean (MB/s) | k6 small p95 (ms) | iperf3 Mem Peak | HTTP Mem Peak |")
        lines.append("|---------|------------------:|------------------------------:|--------------------------------:|-----------------:|------------------:|----------------:|--------------:|")
        for variant in shared:
            lat_mean = mean(latency[variant]["success"]) if latency[variant]["success"] else float("nan")
            ip_up = percentile(iperf3[variant]["upload"], 0.50) / 1e6 if iperf3[variant]["upload"] else float("nan")
            ip_down = percentile(iperf3[variant]["download"], 0.50) / 1e6 if iperf3[variant]["download"] else float("nan")
            http_mean = mean(http[variant]["speed_bps"]) / (1024**2) if http[variant]["speed_bps"] else float("nan")
            k6_small = mean([r['p95_ms'] for r in k6.get(variant, {}).get('small', []) if r['p95_ms'] is not None])
            ip_mem_peak = stats.get((variant, "iperf3"), {}).get("mem_max_bytes", float("nan"))
            http_mem_peak = stats.get((variant, "http"), {}).get("mem_max_bytes", float("nan"))
            lines.append(
                f"| {variant:<8} | {safe_fmt(lat_mean)} | {safe_fmt(ip_up)} | {safe_fmt(ip_down)} | {safe_fmt(http_mean, digits=2)} | {safe_fmt(k6_small)} | {human_bytes(ip_mem_peak)} | {human_bytes(http_mem_peak)} |"
            )
        lines.append("")

    lines.append("*Generated by scripts/analyze_bench.py — cfwarp-cli*")
    return "\n".join(lines) + "\n"


def main() -> None:
    args = parse_args()
    results_dir = Path(args.results_dir)
    ts = args.timestamp
    latency_path = results_dir / f"latency_{ts}.csv"
    iperf3_path = results_dir / f"throughput_iperf3_{ts}.csv"
    http_path = results_dir / f"throughput_http_{ts}.csv"
    k6_path = results_dir / f"k6_summary_{ts}.csv"
    stats_path = results_dir / f"container_stats_{ts}.csv"

    for path in [latency_path, iperf3_path, http_path, k6_path, stats_path]:
        if not path.exists():
            raise SystemExit(f"missing input CSV: {path}")

    latency = collect_latency(load_csv(latency_path))
    iperf3 = collect_iperf3(load_csv(iperf3_path))
    http = collect_http(load_csv(http_path))
    k6 = collect_k6(load_csv(k6_path))
    stats_grouped = collect_stats(load_csv(stats_path))
    stats_summary, stats_raw = summarize_stats(stats_grouped)

    markdown = build_markdown(args, latency, iperf3, http, k6, stats_summary)
    output = Path(args.output) if args.output else results_dir / f"report_{ts}.md"
    json_output = Path(args.json_output) if args.json_output else results_dir / f"summary_{ts}.json"
    output.write_text(markdown)

    summary = {
        "timestamp": ts,
        "proxy_host": args.proxy_host,
        "bench_server": args.bench_server,
        "microwarp_image": args.microwarp_image,
        "latency": latency,
        "iperf3": {
            variant: {
                "upload": section["upload"],
                "download": section["download"],
                "upload_retransmits": section["upload_retransmits"],
                "download_retransmits": section["download_retransmits"],
                "errors": section["errors"],
                "runs": {run: sorted(list(dirs)) for run, dirs in section["runs"].items()},
            }
            for variant, section in iperf3.items()
        },
        "http": http,
        "k6": k6,
        "resource_usage_summary": {f"{variant}:{phase}": value for (variant, phase), value in stats_summary.items()},
        "resource_usage_samples": {f"{variant}:{phase}": value for (variant, phase), value in stats_raw.items()},
        "report_path": str(output),
    }
    json_output.write_text(json.dumps(summary, indent=2, sort_keys=True))
    print(f"wrote {output}")
    print(f"wrote {json_output}")


if __name__ == "__main__":
    main()
