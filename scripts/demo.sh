#!/bin/bash
# Lookout CLI demo script for asciinema recording
#
# Prerequisites:
#   brew install asciinema
#   brew install svg-term-cli   # optional: convert to inline SVG for README
#
# Usage:
#   asciinema rec demo.cast --command ./scripts/demo.sh
#
# Convert to SVG (embed directly in README, no external hosting):
#   svg-term --in demo.cast --out docs/demo.svg --window --width 100 --height 40
#
# Then add to README.md:
#   ![Lookout demo](docs/demo.svg)

set -e

LOOKOUT="${LOOKOUT:-lookout}"
EXAMPLES="${EXAMPLES:-examples}"

# Helpers
type_cmd() {
    echo -n "$ "
    echo "$@" | pv -qL 40   # type effect via pv (brew install pv)
    sleep 0.3
}

run() {
    echo -e "\033[1;32m\$ $*\033[0m"
    sleep 0.5
    eval "$@"
}

pause() {
    sleep "${1:-1.5}"
}

clear

# ── Header ──────────────────────────────────────────────────────────────────
echo -e "\033[1;36m╔══════════════════════════════════════════╗"
echo -e "║        Lookout — CVE & SBOM Analysis     ║"
echo -e "╚══════════════════════════════════════════╝\033[0m"
pause 2

# ── 1. Version ───────────────────────────────────────────────────────────────
echo -e "\n\033[1;33m# Check version\033[0m"
run "$LOOKOUT version"
pause 2

# ── 2. Single CVE lookup ─────────────────────────────────────────────────────
echo -e "\n\033[1;33m# Look up Log4Shell (CVE-2021-44228)\033[0m"
run "$LOOKOUT cve CVE-2021-44228"
pause 3

# ── 3. Batch CVE file ────────────────────────────────────────────────────────
echo -e "\n\033[1;33m# Process a batch CVE file\033[0m"
run "cat $EXAMPLES/text-file-example.txt"
pause 1
run "$LOOKOUT cve-file $EXAMPLES/text-file-example.txt"
pause 3

# ── 4. SBOM scan ─────────────────────────────────────────────────────────────
echo -e "\n\033[1;33m# Scan a CycloneDX SBOM for vulnerabilities\033[0m"
run "$LOOKOUT --severity high sbom $EXAMPLES/cyclonedx-sbom-example.json"
pause 3

# ── 5. Severity filter ───────────────────────────────────────────────────────
echo -e "\n\033[1;33m# Filter to critical only\033[0m"
run "$LOOKOUT --severity critical sbom $EXAMPLES/cyclonedx-sbom-example.json"
pause 3

echo -e "\n\033[1;36m✅  Done. See https://github.com/timoniersystems/lookout\033[0m\n"
