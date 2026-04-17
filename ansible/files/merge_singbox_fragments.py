#!/usr/bin/env python3
import argparse
import json
from pathlib import Path


def upsert_by_tag(items, obj):
    tag = obj.get("tag")
    for idx, item in enumerate(items):
        if isinstance(item, dict) and item.get("tag") == tag:
            items[idx] = obj
            return
    items.append(obj)


def ensure_dict(value, name):
    if value is None:
        return {}
    if not isinstance(value, dict):
        raise SystemExit(f"{name} must be a JSON object")
    return value


def ensure_list(value, name):
    if value is None:
        return []
    if not isinstance(value, list):
        raise SystemExit(f"{name} must be a JSON array")
    return value


def load_json(path: Path):
    return json.loads(path.read_text())


def write_json(path: Path, data):
    path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n")


def merge_outbounds(data, warp_port, masque_port, warp_tag, masque_tag):
    data = ensure_dict(data, "outbounds document")
    outbounds = ensure_list(data.get("outbounds"), "outbounds")
    upsert_by_tag(
        outbounds,
        {
            "type": "socks",
            "tag": warp_tag,
            "server": "127.0.0.1",
            "server_port": warp_port,
            "version": "5",
            "domain_resolver": {
                "server": "dns",
                "strategy": "ipv4_only",
            },
        },
    )
    upsert_by_tag(
        outbounds,
        {
            "type": "socks",
            "tag": masque_tag,
            "server": "127.0.0.1",
            "server_port": masque_port,
            "version": "5",
            "domain_resolver": {
                "server": "dns",
                "strategy": "ipv4_only",
            },
        },
    )
    data["outbounds"] = outbounds
    return data


def is_non_final_route_action(rule):
    if not isinstance(rule, dict):
        return False
    return rule.get("action") in {"sniff", "resolve", "route-options"}


def merge_routes(data, rule_set_tag, rule_set_url, rule_target_outbound):
    data = ensure_dict(data, "routes document")
    route = ensure_dict(data.get("route"), "route")

    rule_sets = ensure_list(route.get("rule_set"), "route.rule_set")
    upsert_by_tag(
        rule_sets,
        {
            "tag": rule_set_tag,
            "type": "remote",
            "format": "binary",
            "url": rule_set_url,
            "update_interval": "1d",
        },
    )

    rules = ensure_list(route.get("rules"), "route.rules")
    filtered_rules = []
    for rule in rules:
        if not isinstance(rule, dict):
            filtered_rules.append(rule)
            continue
        existing = rule.get("rule_set") or []
        if isinstance(existing, str):
            existing = [existing]
        if rule_set_tag in existing:
            continue
        filtered_rules.append(rule)

    managed_rule = {
        "rule_set": [rule_set_tag],
        "action": "route",
        "outbound": rule_target_outbound,
    }

    prefix = []
    suffix = filtered_rules
    while suffix and is_non_final_route_action(suffix[0]):
        prefix.append(suffix.pop(0))

    route["rule_set"] = rule_sets
    route["rules"] = prefix + [managed_rule] + suffix
    data["route"] = route
    return data


def main() -> int:
    ap = argparse.ArgumentParser(description="Merge cfwarp dogfood outbounds/routes into sing-box fragment files")
    ap.add_argument("--outbounds", required=True)
    ap.add_argument("--routes", required=True)
    ap.add_argument("--warp-port", type=int, required=True)
    ap.add_argument("--masque-port", type=int, required=True)
    ap.add_argument("--warp-tag", default="cfwarp-warp-local")
    ap.add_argument("--masque-tag", default="cfwarp-masque-local")
    ap.add_argument("--rule-set-tag", default="geosite-google-deepmind")
    ap.add_argument(
        "--rule-set-url",
        default="https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-google-deepmind.srs",
    )
    ap.add_argument("--rule-target-outbound", default="cfwarp-masque-local")
    ap.add_argument("--outbounds-output", required=True)
    ap.add_argument("--routes-output", required=True)
    args = ap.parse_args()

    outbounds = merge_outbounds(
        load_json(Path(args.outbounds)),
        args.warp_port,
        args.masque_port,
        args.warp_tag,
        args.masque_tag,
    )
    routes = merge_routes(
        load_json(Path(args.routes)),
        args.rule_set_tag,
        args.rule_set_url,
        args.rule_target_outbound,
    )

    write_json(Path(args.outbounds_output), outbounds)
    write_json(Path(args.routes_output), routes)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
