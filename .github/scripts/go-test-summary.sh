#!/usr/bin/env bash
# Runs the Go suite once, appends pass/fail/skip counts and total statement
# coverage to the GitHub Actions job summary, and exits with the suite's own
# status so a red test still fails the job. Arg $1: optional label (e.g. the OS)
# appended to the heading. Falls back to stdout when run outside Actions.
set -uo pipefail

label="${1:+ — $1}"
json="$(mktemp)"
cover="$(mktemp)"

started=$SECONDS
go test -json -coverprofile="$cover" ./... >"$json"
code=$?
wall=$((SECONDS - started))

read -r pass fail skip < <(jq -rs '
  map(select(.Test != null and (.Action == "pass" or .Action == "fail" or .Action == "skip")))
  | [ (map(select(.Action == "pass")) | length),
      (map(select(.Action == "fail")) | length),
      (map(select(.Action == "skip")) | length) ] | @tsv
' "$json")

# -json sends every line of test output to the file, not the log, so a red run
# would otherwise leave the job with an exit code and nothing to read — the
# summary lands in the job summary, which the log does not carry. Replay the
# failing tests' own output (plus their package's, where a build failure lands)
# to the log before writing the summary.
if [ "$code" != "0" ]; then
  echo "::group::Failed test output"
  # -j: the captured Output lines already carry their newline.
  jq -js '
    [ .[] | select(.Action == "fail") | .Package ] as $failed
    | .[]
    | select(.Action == "output" and (.Package | IN($failed[])))
    | .Output
  ' "$json"
  echo "::endgroup::"
fi

# The root package is the main bootstrap invariant #1 exempts; its logic lives
# in internal/, so its files leave the denominator. Every other boundary is counted.
grep -v '^github.com/omartelo/lich/[^/]*\.go:' "$cover" >"$cover.gated"
total="$(go tool cover -func="$cover.gated" | awk '/^total:/ {print $3}')"

min=80
slowest=15
if ! awk -v t="${total%\%}" -v m="$min" 'BEGIN { exit !(t != "" && t + 0 >= m) }'; then
  echo "::error::backend coverage ${total:-n/a} is below the ${min}% bar (CLAUDE.md invariant 1)"
  [ "$code" = "0" ] && code=1
fi

summary="${GITHUB_STEP_SUMMARY:-/dev/stdout}"
{
  echo "### Backend (Go)${label}"
  echo ""
  echo "| Passed | Failed | Skipped | Coverage |"
  echo "|-------:|-------:|--------:|---------:|"
  echo "| ${pass:-0} | ${fail:-0} | ${skip:-0} | ${total:-n/a} |"
  echo ""
  if [ "${fail:-0}" != "0" ]; then
    echo "<details><summary>Failed tests</summary>"
    echo ""
    jq -rs 'map(select(.Action == "fail" and .Test != null)) | .[] | "- `\(.Package) \(.Test)`"' "$json" | sort -u
    echo ""
    echo "</details>"
  fi
  # Elapsed is run time only: the gap between the wall time and the slowest
  # package is compiling and linking the test binaries, which is where one OS
  # can lose minutes to another without any test getting slower.
  echo "<details><summary>Timings: ${wall}s wall</summary>"
  echo ""
  echo "| Package | Seconds |"
  echo "|:--|--:|"
  jq -rs --argjson n "$slowest" '
    map(select(.Test == null and .Elapsed != null and (.Action == "pass" or .Action == "fail")))
    | sort_by(-.Elapsed) | .[:$n][] | "| `\(.Package)` | \(.Elapsed) |"
  ' "$json"
  echo ""
  echo "| Test | Seconds |"
  echo "|:--|--:|"
  jq -rs --argjson n "$slowest" '
    map(select(.Test != null and (.Test | contains("/") | not) and .Elapsed != null and (.Action == "pass" or .Action == "fail")))
    | sort_by(-.Elapsed) | .[:$n][] | "| `\(.Package | sub(".*/"; "")).\(.Test)` | \(.Elapsed) |"
  ' "$json"
  echo ""
  echo "</details>"
} >>"$summary"

exit "$code"
