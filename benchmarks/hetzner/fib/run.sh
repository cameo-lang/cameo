#!/usr/bin/env bash
# Times fib on cameo binaries at several core counts and on the CRubies
# provision.sh built, and prints a TSV row per case: the best of three
# processes' in-script seconds, the largest peak RSS in MB, and the result.
#
#   benchmarks/hetzner/fib/run.sh CAMEO_BINARY...
#
# CORES sets the core counts (default "1 4 16"); each cameo run is pinned
# to that many CPUs with taskset and GOMAXPROCS. RUBIES lists the
# /opt/ruby-* versions to time (default: all), each without a JIT, with
# YJIT and, from 4.0, with ZJIT. seq.rb and list.rb are plain Ruby;
# async.rb and coarse.rb use cameo's async, and run on cameo alone.
set -euo pipefail
cd "$(dirname "$0")"

CORES=${CORES:-"1 4 16"}

# best runs a command three times and prints the best seconds, the largest
# RSS and the result.
best() {
  local secs="" rss=0 res="" out r t m
  for _ in 1 2 3; do
    out=$( { /usr/bin/time -f "RSS %M" "$@"; } 2>&1 )
    read -r r t <<<"$(grep -v '^RSS' <<<"$out")"
    m=$(awk '/^RSS/{print int($2/1024)}' <<<"$out")
    (( m > rss )) && rss=$m
    res=$r
    if [[ -z $secs ]] || awk "BEGIN{exit !($t < $secs)}"; then secs=$t; fi
  done
  printf "%.3f\t%s\t%s" "$secs" "$rss" "$res"
}

printf "who\tcores\tscript\tsecs\tmaxrss_mb\tresult\n"
for bin in "$@"; do
  for script in seq.rb list.rb coarse.rb async.rb; do
    for c in $CORES; do
      printf "%s\t%s\t%s\t%s\n" "$(basename "$bin")" "$c" "$script" \
        "$(best taskset -c "0-$((c - 1))" env GOMAXPROCS="$c" "$bin" "$script")"
    done
  done
done
for dir in ${RUBIES:-/opt/ruby-*}; do
  [[ -x $dir/bin/ruby ]] || continue
  v=$("$dir/bin/ruby" -e 'print RUBY_VERSION')
  flags=("" "--yjit")
  [[ ${v%%.*} -ge 4 ]] && flags+=("--zjit")
  for f in "${flags[@]}"; do
    for script in seq.rb list.rb; do
      # shellcheck disable=SC2086
      printf "ruby-%s%s\t1\t%s\t%s\n" "$v" "${f:+ $f}" "$script" "$(best taskset -c 0 "$dir/bin/ruby" $f "$script")"
    done
  done
done
