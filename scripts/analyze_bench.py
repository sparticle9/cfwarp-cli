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
    value = value.strip()
    if not value or value.upper() == "ERR":
        return None
    try:
        return float(value)
    except ValueError:
        return None


def parse_percent(value: str | None) -> float | None:
    if value is None:
        return None
    return to_float(value.replace("%", "").strip())


def parse_size_to_bytes(value: str | None) -> float | None:
    if value is None:
        return None
    s = value.strip()
    if not s or s.upper() == "ERR":
        return None
    if s.endswith("B") and s[:-1].replace(".", "", 1).isdigit():
        return float(s[:-1])

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
            num = to_float(s[: -len(unit)])
            return None if num is None else num * units[unit]
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


def human_bytes(num: float | None) -> str:
    if num is None or math.isnan(num):
        return "n/a"
    value = float(num)
    for unit in ["B", "KiB", "MiB", "GiB", "TiB"]:
        if abs(value) < 1024.0 or unit == "TiB":
            return f"{value:.2f} {unit}"
        value /= 1024.0
    return f"{value:.2f} TiB"


def safe_fmt(value: float | None, digits: int = 1, scale: float = 1.0) -> str:
    if value is None or math.isnan(value):
        return "n/a"
    return f"{value / scale:.{digits}f}"


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
    grouped: dict[str, dict[str, Any]] = defaultdict(lambda: {"success": [], "errors": 0})
    for row in rows:
        variant = row.get("variant", "unknown")
        bps = to_float(row.get("bits_per_second"))
        if bps is None or bps <= 0:
            grouped[variant]["errors"] += 1
        else:
            grouped[variant]["success"].append(bps)
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


def collect_stats(rows: list[dict[str, str]]) -> dict[tuple[str, str], dict[str, Any]]:
    grouped: dict[tuple[str, str], dict[str, Any]] = defaultdict(lambda: {
        "samples": 0,
        "cpu": [],
        "mem_used": [],
        "mem_limit": [],
        "net_rx": [],
        "net_tx": [],
        "timestamps": [],
    })

    for row in rows:
        variant = row.get("variant", "unknown")
        phase = row.get("phase", "unknown")
        entry = grouped[(variant, phase)]
        entry["samples"] += 1
        cpu = parse_percent(row.get("cpu_pct"))
        if cpu is not None:
            entry["cpu"].append(cpu)
        mem_used = parse_size_to_bytes(row.get("mem_used"))
        mem_limit = parse_size_to_bytes(row.get("mem_limit"))
        net_rx = parse_size_to_bytes(row.get("net_rx"))
        net_tx = parse_size_to_bytes(row.get("net_tx"))
        if mem_used is not None:
            entry["mem_used"].append(mem_used)
        if mem_limit is not None:
            entry["mem_limit"].append(mem_limit)
        if net_rx is not None:
            entry["net_rx"].append(net_rx)
        if net_tx is not None:
            entry["net_tx"].append(net_tx)
        ts = row.get("sample_ts")
        if ts:
            tsf = to_float(ts)
            if tsf is not None:
                entry["timestamps"].append(tsf)
    return grouped


def summarize_stats(grouped: dict[tuple[str, str], dict[str, Any]]) -> dict[tuple[str, str], dict[str, Any]]:
    summary: dict[tuple[str, str], dict[str, Any]] = {}
    for key, entry in grouped.items():
        cpu = entry["cpu"]
        mem = entry["mem_used"]
        mem_limit = entry["mem_limit"]
        rx = entry["net_rx"]
        tx = entry["net_tx"]
        summary[key] = {
            "samples": entry["samples"],
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
    return summary


def common_variants(*sections: dict[str, Any]) -> list[str]:
    keys = None
    for section in sections:
        present = set(section)
        keys = present if keys is None else keys & present
    return sorted(keys or [])


def build_markdown(args: argparse.Namespace, latency: dict[str, Any], iperf3: dict[str, Any], http: dict[str, Any], stats: dict[tuple[str, str], Any]) -> str:
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
    lines.append(f"| Container CPU | {args.container_cpus} vCPU |")
    lines.append(f"| Container memory | {args.container_memory} |")
    lines.append(f"| iperf3 | {args.iperf3_parallel} streams × {args.iperf3_duration}s |")
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

    lines.append("## Throughput — iperf3 raw WireGuard (Mbit/s)")
    lines.append("")
    lines.append("| Variant | Median | p95 | Max | Mean | Success | Errors |")
    lines.append("|---------|-------:|----:|----:|-----:|--------:|-------:|")
    for variant in sorted(iperf3):
        vals = iperf3[variant]["success"]
        errs = iperf3[variant]["errors"]
        lines.append(
            f"| {variant:<8} | {safe_fmt(percentile(vals, 0.50) if vals else float('nan'), scale=1e6)} | "
            f"{safe_fmt(percentile(vals, 0.95) if vals else float('nan'), scale=1e6)} | "
            f"{safe_fmt(max(vals) if vals else float('nan'), scale=1e6)} | "
            f"{safe_fmt(mean(vals) if vals else float('nan'), scale=1e6)} | {len(vals)} | {errs} |"
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

    lines.append("## Resource Usage (docker stats)")
    lines.append("")
    lines.append("| Variant | Phase | Samples | CPU Mean % | CPU p95 % | CPU Max % | Mem Mean | Mem Peak | Net RX Δ | Net TX Δ |")
    lines.append("|---------|-------|--------:|-----------:|----------:|----------:|---------:|---------:|---------:|---------:|")
    for (variant, phase) in sorted(stats):
        entry = stats[(variant, phase)]
        lines.append(
            f"| {variant:<8} | {phase:<7} | {entry['samples']} | "
            f"{safe_fmt(entry['cpu_mean'], digits=2)} | {safe_fmt(entry['cpu_p95'], digits=2)} | {safe_fmt(entry['cpu_max'], digits=2)} | "
            f"{human_bytes(entry['mem_mean_bytes'])} | {human_bytes(entry['mem_max_bytes'])} | "
            f"{human_bytes(entry['net_rx_delta_bytes'])} | {human_bytes(entry['net_tx_delta_bytes'])} |"
        )
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## Efficiency — Throughput vs container cost")
    lines.append("")
    lines.append("| Variant | iperf3 Median (Mbit/s) | iperf3 CPU Mean % | iperf3 CPU Max % | iperf3 Mem Peak | Mbit/s per CPU% | HTTP Mean (MB/s) | HTTP CPU Mean % |")
    lines.append("|---------|----------------------:|------------------:|-----------------:|----------------:|----------------:|----------------:|----------------:|")
    for variant in sorted(set(list(iperf3.keys()) + list(http.keys()))):
        ip_vals = iperf3.get(variant, {}).get("success", [])
        http_vals = http.get(variant, {}).get("speed_bps", [])
        ip_stats = stats.get((variant, "iperf3"), {})
        http_stats = stats.get((variant, "http"), {})
        ip_median_mbps = percentile(ip_vals, 0.50) / 1e6 if ip_vals else float("nan")
        ip_cpu_mean = ip_stats.get("cpu_mean", float("nan"))
        ip_cpu_max = ip_stats.get("cpu_max", float("nan"))
        ip_mem_peak = ip_stats.get("mem_max_bytes", float("nan"))
        eff = ip_median_mbps / ip_cpu_mean if ip_vals and ip_cpu_mean and not math.isnan(ip_cpu_mean) and ip_cpu_mean > 0 else float("nan")
        http_mean_mibs = mean(http_vals) / (1024**2) if http_vals else float("nan")
        http_cpu_mean = http_stats.get("cpu_mean", float("nan"))
        lines.append(
            f"| {variant:<8} | {safe_fmt(ip_median_mbps)} | {safe_fmt(ip_cpu_mean, digits=2)} | {safe_fmt(ip_cpu_max, digits=2)} | {human_bytes(ip_mem_peak)} | {safe_fmt(eff, digits=2)} | {safe_fmt(http_mean_mibs, digits=2)} | {safe_fmt(http_cpu_mean, digits=2)} |"
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
            anomalies.append(f"- {variant}: iperf3 failed runs={section['errors']}")
    for variant, section in sorted(http.items()):
        bad_codes = sorted({code for code in section["codes"] if code and code != "200"})
        if section["errors"]:
            suffix = f" (http codes: {', '.join(bad_codes)})" if bad_codes else ""
            anomalies.append(f"- {variant}: HTTP failed runs={section['errors']}{suffix}")
    if not anomalies:
        anomalies.append("- No failed runs recorded in the fetched CSVs.")
    lines.extend(anomalies)
    lines.append("")

    shared = common_variants(latency, iperf3, http)
    if len(shared) >= 2 and {"alpine", "debian"}.issubset(shared):
        lines.append("---")
        lines.append("")
        lines.append("## Alpine vs Debian quick comparison")
        lines.append("")
        def delta_pct(new: float, old: float) -> str:
            if any(math.isnan(x) for x in [new, old]) or old == 0:
                return "n/a"
            return f"{((new - old) / old) * 100:+.1f}%"

        a_lat = mean(latency["alpine"]["success"]) if latency["alpine"]["success"] else float("nan")
        d_lat = mean(latency["debian"]["success"]) if latency["debian"]["success"] else float("nan")
        a_ip = percentile(iperf3["alpine"]["success"], 0.50) / 1e6 if iperf3["alpine"]["success"] else float("nan")
        d_ip = percentile(iperf3["debian"]["success"], 0.50) / 1e6 if iperf3["debian"]["success"] else float("nan")
        a_http = mean(http["alpine"]["speed_bps"]) / (1024**2) if http["alpine"]["speed_bps"] else float("nan")
        d_http = mean(http["debian"]["speed_bps"]) / (1024**2) if http["debian"]["speed_bps"] else float("nan")
        lines.append("| Metric | Alpine | Debian | Debian vs Alpine |")
        lines.append("|--------|------:|-------:|-----------------:|")
        lines.append(f"| Mean latency (ms) | {safe_fmt(a_lat)} | {safe_fmt(d_lat)} | {delta_pct(d_lat, a_lat)} |")
        lines.append(f"| iperf3 median (Mbit/s) | {safe_fmt(a_ip)} | {safe_fmt(d_ip)} | {delta_pct(d_ip, a_ip)} |")
        lines.append(f"| HTTP mean (MB/s) | {safe_fmt(a_http, digits=2)} | {safe_fmt(d_http, digits=2)} | {delta_pct(d_http, a_http)} |")
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
    stats_path = results_dir / f"container_stats_{ts}.csv"

    for path in [latency_path, iperf3_path, http_path, stats_path]:
        if not path.exists():
            raise SystemExit(f"missing input CSV: {path}")

    latency = collect_latency(load_csv(latency_path))
    iperf3 = collect_iperf3(load_csv(iperf3_path))
    http = collect_http(load_csv(http_path))
    stats = summarize_stats(collect_stats(load_csv(stats_path)))

    markdown = build_markdown(args, latency, iperf3, http, stats)
    output = Path(args.output) if args.output else results_dir / f"report_{ts}.md"
    json_output = Path(args.json_output) if args.json_output else results_dir / f"summary_{ts}.json"
    output.write_text(markdown)

    summary = {
        "timestamp": ts,
        "proxy_host": args.proxy_host,
        "bench_server": args.bench_server,
        "latency": latency,
        "iperf3": iperf3,
        "http": http,
        "resource_usage": {f"{variant}:{phase}": value for (variant, phase), value in stats.items()},
        "report_path": str(output),
    }
    json_output.write_text(json.dumps(summary, indent=2, sort_keys=True))
    print(f"wrote {output}")
    print(f"wrote {json_output}")


if __name__ == "__main__":
    main()
