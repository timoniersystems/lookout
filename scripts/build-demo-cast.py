#!/usr/bin/env python3
"""
Build an asciinema cast file for Lookout CLI demo from real command output.

Usage:
    # 1. Capture fresh command outputs (requires lookout binary + Dgraph for dep-path):
    ./lookout cve CVE-2021-44228 2>/dev/null > /tmp/cve_out.txt
    ./lookout cve-file examples/text-file-example.txt 2>/dev/null > /tmp/cvefile_out.txt
    ./lookout --severity critical sbom examples/cyclonedx-sbom-example.json 2>/dev/null > /tmp/sbom_out.txt
    make up-standalone && sleep 5
    DGRAPH_HOST=localhost ./lookout sbom examples/cyclonedx-sbom-example.json \
        --dep-path 'pkg:composer/asm89/stack-cors@1.3.0' 2>/dev/null > /tmp/deppath_out.txt
    make down

    # 2. Build cast:
    python3 scripts/build-demo-cast.py

    # 3. Render to SVG:
    svg-term --in /tmp/lookout-demo.cast --out docs/demo.svg --window --width 100 --height 40
"""

import json

COLS, ROWS = 100, 40

def output(text, t, line_delay=0.07):
    events = []
    for line in text.splitlines(keepends=True):
        events.append([round(t, 4), "o", line])
        t += line_delay
    return events, t

def prompt(t):
    return [[round(t, 4), "o", "\033[1;32m$ \033[0m"]], t

def type_cmd(cmd, t):
    events, t = prompt(t)
    for ch in cmd:
        events.append([round(t, 4), "o", ch])
        t += 0.07
    events.append([round(t, 4), "o", "\r\n"])
    return events, t + 0.5

def comment(text, t):
    e = [[round(t, 4), "o", f"\033[2;37m# {text}\033[0m\r\n"]]
    return e, t + 0.4

def blank(t):
    return [[round(t, 4), "o", "\r\n"]], t + 0.1

def trim(text, max_lines):
    """Keep first max_lines lines, append a dimmed ellipsis if truncated."""
    lines = text.splitlines(keepends=True)
    if len(lines) <= max_lines:
        return text
    kept = "".join(lines[:max_lines])
    return kept + "\033[2m  ... (truncated for brevity)\033[0m\r\n"

# Read captured outputs
with open("/tmp/cve_out.txt") as f: cve_out = f.read()
with open("/tmp/cvefile_out.txt") as f: cvefile_out = f.read()
with open("/tmp/sbom_out.txt") as f: sbom_out = f.read()
with open("/tmp/deppath_out.txt") as f: deppath_out = f.read()

# Trim long outputs to avoid wall-of-text scrolling
cve_out     = trim(cve_out, 32)
cvefile_out = trim(cvefile_out, 35)
sbom_out    = trim(sbom_out, 38)

events = []
t = 0.8

# Header
hdr  = "\033[1;36m╔══════════════════════════════════════════════╗\r\n"
hdr += "║     Lookout — CVE & SBOM Analysis Tool       ║\r\n"
hdr += "╚══════════════════════════════════════════════╝\033[0m\r\n"
e, t = output(hdr, t, 0.08); events += e
t += 1.5

# 1. version
e, t = comment("Show version", t); events += e
e, t = type_cmd("lookout version", t); events += e
e, t = output("lookout version v0.9.5\r\n", t, 0.07); events += e
t += 2.0

# 2. single CVE lookup
e, t = blank(t); events += e
e, t = comment("Look up Log4Shell (CVE-2021-44228)", t); events += e
e, t = type_cmd("lookout cve CVE-2021-44228", t); events += e
e, t = output(cve_out.replace("\n", "\r\n"), t, 0.06); events += e
t += 3.0

# 3. batch CVE file
e, t = blank(t); events += e
e, t = comment("Process a batch CVE list", t); events += e
e, t = type_cmd("cat examples/text-file-example.txt", t); events += e
e, t = output("CVE-2021-36159\r\nCVE-2021-30139\r\n", t, 0.07); events += e
t += 0.8
e, t = type_cmd("lookout cve-file examples/text-file-example.txt", t); events += e
e, t = output(cvefile_out.replace("\n", "\r\n"), t, 0.055); events += e
t += 3.0

# 4. SBOM scan — critical only
e, t = blank(t); events += e
e, t = comment("Scan a CycloneDX SBOM — critical vulnerabilities only", t); events += e
e, t = type_cmd("lookout --severity critical sbom examples/cyclonedx-sbom-example.json", t); events += e
e, t = output(sbom_out.replace("\n", "\r\n"), t, 0.05); events += e
t += 3.0

# 5. dependency path traversal
e, t = blank(t); events += e
e, t = comment("Trace how a vulnerable package entered the project", t); events += e
e, t = type_cmd(
    "lookout sbom examples/cyclonedx-sbom-example.json \\\r\n"
    "    --dep-path 'pkg:composer/asm89/stack-cors@1.3.0'", t
); events += e
e, t = output(deppath_out.replace("\n", "\r\n"), t, 0.07); events += e
t += 3.0

# Footer
e, t = blank(t); events += e
e, t = output("\033[1;36m✅  https://github.com/timoniersystems/lookout\033[0m\r\n", t, 0.07)
events += e
t += 1.5

header = {
    "version": 2,
    "width": COLS,
    "height": ROWS,
    "timestamp": 1773177000,
    "title": "Lookout — CVE & SBOM vulnerability analysis",
    "env": {"TERM": "xterm-256color", "SHELL": "/bin/bash"}
}

out = "/tmp/lookout-demo.cast"
with open(out, "w") as f:
    f.write(json.dumps(header) + "\n")
    for ev in events:
        f.write(json.dumps(ev) + "\n")

print(f"Cast written to {out}: {len(events)} events, {round(t, 1)}s duration")
print("Next: svg-term --in /tmp/lookout-demo.cast --out docs/demo.svg --window --width 100 --height 40")
