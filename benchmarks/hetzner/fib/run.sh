#!/usr/bin/env bash
# Times fib on cameo binaries at several core counts and on the CRubies
# provision.sh built, and prints a TSV row per case: the best of three
# processes' in-script seconds, the largest peak RSS in MB, and the result.
#
#   benchmarks/hetzner/fib/run.sh CAMEO_BINARY...
#
# CORES sets the core counts (default "1 4 16"); each cameo run is pinned
# to that many CPUs with taskset and GOMAXPROCS. RUBIES lists the Ruby
# install directories to time (default: /opt/ruby-*). seq.rb and list.rb
# are plain Ruby; async.rb and coarse.rb use cameo's async, and run on
# cameo alone. A run that fails stops the script.
set -euo pipefail

bins=()
for bin in "$@"; do bins+=("$(realpath "$bin")"); done
cd "$(dirname "$0")"

CORES=${CORES:-"1 4 16"}
rss_file=$(mktemp)
trap 'rm -f "$rss_file"' EXIT

# best runs a command three times and prints the best seconds, the largest
# RSS and the result. The command prints "result seconds" as its last line
# on stdout; its stderr goes to run.sh's.
best() {
  local secs="" rss=0 res="" out r t m
  for _ in 1 2 3; do
    if ! out=$(/usr/bin/time -o "$rss_file" -f "%M" "$@"); then
      echo "run.sh: failed: $*" >&2
      return 1
    fi
    read -r r t <<<"$(tail -n 1 <<<"$out")"
    m=$(( $(tail -n 1 "$rss_file") / 1024 ))
    (( m > rss )) && rss=$m
    res=$r
    if [[ -z $secs ]] || awk "BEGIN{exit !($t < $secs)}"; then secs=$t; fi
  done
  printf "%.3f\t%s\t%s" "$secs" "$rss" "$res"
}

printf "who\tcores\tscript\tsecs\tmaxrss_mb\tresult\n"
for bin in ${bins[@]+"${bins[@]}"}; do
  for script in seq.rb list.rb coarse.rb async.rb; do
    for c in $CORES; do
      row=$(best taskset -c "0-$((c - 1))" env GOMAXPROCS="$c" "$bin" "$script")
      printf "%s\t%s\t%s\t%s\n" "$(basename "$bin")" "$c" "$script" "$row"
    done
  done
done
for dir in ${RUBIES:-/opt/ruby-*}; do
  [[ -x $dir/bin/ruby ]] || continue
  v=$("$dir/bin/ruby" -e 'print RUBY_VERSION')
  for script in seq.rb list.rb; do
    row=$(best taskset -c 0 "$dir/bin/ruby" "$script")
    printf "ruby-%s\t1\t%s\t%s\n" "$v" "$script" "$row"
  done
done
