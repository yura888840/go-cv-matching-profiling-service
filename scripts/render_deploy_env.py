#!/usr/bin/env python3
"""Render systemd EnvironmentFile without leaking secrets to logs."""
import ipaddress
import json
import os
import re
import sys

def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: render_deploy_env.py OUTPUT_PATH")
    keys = {
        "DATABASE_URL": os.getenv("DATABASE_URL", ""),
        "API_KEY": os.getenv("API_KEY", ""),
        "ALLOWED_IPS": os.getenv("ALLOWED_IPS", ""),
        "ALLOWED_DOMAINS": os.getenv("ALLOWED_DOMAINS", ""),
        "MATCHING_WORKERS": os.getenv("MATCHING_WORKERS", "4"),
        "MAX_CVS": os.getenv("MAX_CVS", "500000"),
        "PORT": os.getenv("APP_PORT", "8080"),
    }
    if not keys["DATABASE_URL"]:
        raise SystemExit("Missing DATABASE_URL GitHub secret")
    if len(keys["API_KEY"]) < 32:
        raise SystemExit("Missing/short API_KEY GitHub secret")
    ips = [s.strip() for s in keys["ALLOWED_IPS"].split(",") if s.strip()]
    if not ips:
        raise SystemExit("ALLOWED_IPS must contain client IPs/CIDRs and 127.0.0.1")
    for entry in ips:
        try:
            ipaddress.ip_network(entry, strict=False)
        except ValueError:
            raise SystemExit("Invalid ALLOWED_IPS entry") from None
    if not any(ipaddress.ip_address("127.0.0.1") in ipaddress.ip_network(entry, strict=False) for entry in ips):
        raise SystemExit("ALLOWED_IPS must include 127.0.0.1 for local readiness checks")
    for host in [s.strip() for s in keys["ALLOWED_DOMAINS"].split(",") if s.strip()]:
        if not re.fullmatch(r"(?i)[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?", host):
            raise SystemExit("Invalid ALLOWED_DOMAINS entry")
    for name, low, high in [("MATCHING_WORKERS", 1, 64), ("MAX_CVS", 1, 100000000), ("PORT", 1, 65535)]:
        try:
            value = int(keys[name])
        except ValueError:
            raise SystemExit(f"{name} must be an integer") from None
        if not low <= value <= high:
            raise SystemExit(f"{name} is out of range")
    if any("\n" in val or "\r" in val or "\x00" in val for val in keys.values()):
        raise SystemExit("Environment values must be single-line strings")
    # systemd EnvironmentFile supports double-quoted values; JSON escapes
    # most special characters but backslash escaping differs for certain
    # sequences. Use shell-style quoting with escaped backslashes/quotes.
    def quote(value: str) -> str:
        return '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'
    with open(sys.argv[1], "w", encoding="utf-8") as f:
        for key, value in keys.items():
            f.write(f"{key}={quote(value)}\n")
    os.chmod(sys.argv[1], 0o600)
    print("Production environment file prepared (values redacted).")

if __name__ == "__main__":
    main()
