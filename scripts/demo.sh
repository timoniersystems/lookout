#!/bin/bash
# Lookout CLI demo script for asciinema recording
#
# Prerequisites:
#   brew install asciinema pv
#   npm install -g svg-term-cli
#   make up-standalone        # start Dgraph (needed for --dep-path)
#   export DGRAPH_HOST=localhost
#
# Usage:
#   asciinema rec demo.cast --command ./scripts/demo.sh
#
# Convert to inline SVG for README (no external hosting):
#   svg-term --in demo.cast --out docs/demo.svg --window --width 100 --height 40

set -e

LOOKOUT="${LOOKOUT:-lookout}"
EXAMPLES="${EXAMPLES:-examples}"

run() {
    echo -e "\033[1;32m\$ $*\033[0m"
    sleep 0.5
    eval "$@"
}

pause() { sleep "${1:-1.5}"; }

clear

# ── Header ────────────────────────────────────────────────────────────────────
echo -e "\033[1;36m╔══════════════════════════════════════════╗"
echo -e "║        Lookout — CVE & SBOM Analysis     ║"
echo -e "╚══════════════════════════════════════════╝\033[0m"
pause 2

# ── 1. Version ────────────────────────────────────────────────────────────────
echo -e "\n\033[2;37m# Check version\033[0m"
run "$LOOKOUT version"
pause 1.5

# ── 2. Single CVE lookup ──────────────────────────────────────────────────────
echo -e "\n\033[2;37m# Look up Log4Shell (CVE-2021-44228)\033[0m"
run "$LOOKOUT cve CVE-2021-44228"
pause 3

# ── 3. Batch CVE file ─────────────────────────────────────────────────────────
echo -e "\n\033[2;37m# Process a batch CVE list\033[0m"
run "cat $EXAMPLES/text-file-example.txt"
pause 1
run "$LOOKOUT cve-file $EXAMPLES/text-file-example.txt"
pause 3

# ── 4. SBOM scan — critical ───────────────────────────────────────────────────
echo -e "\n\033[2;37m# Scan a CycloneDX SBOM for critical vulnerabilities\033[0m"
run "$LOOKOUT --severity critical sbom $EXAMPLES/cyclonedx-sbom-example.json"
pause 3

# ── 5. Dependency path traversal ─────────────────────────────────────────────
echo -e "\n\033[2;37m# Trace how a vulnerable package entered the project\033[0m"
run "$LOOKOUT sbom $EXAMPLES/cyclonedx-sbom-example.json \\
    --dep-path 'pkg:composer/asm89/stack-cors@1.3.0'"
pause 3

echo -e "\n\033[1;36m✅  https://github.com/timoniersystems/lookout\033[0m\n"
