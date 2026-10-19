---
name: hetzner-benchmarks
description: Run cameo's benchmarks on a temporary 16-core Hetzner server - the full benchmarks/compare sweep against MRI, warmruby and rbgo, with a baseline of main first, and the fib async scaling run at 1, 4 and 16 cores against CRuby 3.4 and 4.0. Use when a change needs before-and-after numbers on many cores, or when asked to rerun the recorded benchmarks.
---

# Benchmarks on a Hetzner server

A laptop has too few cores, and cores of two speeds, to show how `async`
scales, and too much going on to time 824 cases quietly. A dedicated
16-core server does both. It costs about €0.53 an hour and a full run takes
about an hour and a half.

## Before you start

- **Ask the human before creating the server**: it spends money. Name the
  type, the location and the expected time. Run one server at a time.
- Build every cameo you compare from a commit, never from a dirty tree.
- The hcloud context `ark-ci` also holds the GitHub CI runners
  (`github-hetzner-runner-*`). Touch only the server you created.

## 1. Create the server

```sh
hcloud server create --name cameo-bench-<topic> --type ccx43 \
  --image ubuntu-24.04 --location hel1 --ssh-key <key id> \
  --label purpose=cameo-bench
```

`ccx43` has 16 dedicated AMD vCPUs; shared types (`cpx`, `cax`) give noisy
times at high core counts. `hcloud ssh-key list` names the keys; use the one
whose fingerprint matches `ssh-keygen -E md5 -lf ~/.ssh/id_rsa.pub`. Keep the
host key out of `~/.ssh/known_hosts`: pass
`-o UserKnownHostsFile=<scratch>/known_hosts -o StrictHostKeyChecking=accept-new`
to ssh and scp, and wait until `ssh root@IP true` succeeds.

## 2. Provision it

```sh
ssh root@IP 'bash -s' < benchmarks/hetzner/provision.sh
```

[provision.sh](../../../benchmarks/hetzner/provision.sh) installs Go at
go.mod's version, and CRuby 3.4 and 4.0 built from source into
`/opt/ruby-<version>`. Ruby 3.4 becomes the
`ruby` on PATH, which benchmarks/compare measures as `mri`. It takes about
ten minutes.

## 3. Copy the repository

The server has no GitHub credentials. Send a bundle of the branches; the
clone has them as `origin/<branch>`, and no local branch to check out:

```sh
git fetch origin
git bundle create <scratch>/cameo.bundle origin/main <branch>...
scp <scratch>/cameo.bundle root@IP:/root/
ssh root@IP 'git clone -q /root/cameo.bundle /root/cameo'
```

## 4. Record the baseline: benchmarks/compare on main

```sh
ssh root@IP 'cd /root/cameo && git checkout -q --detach origin/main &&
  nohup benchmarks/compare/rerun.sh --no-color \
    --out benchmarks/compare/results-main.tsv > /root/compare-main.log 2>&1 &'
```

rerun.sh builds cameo from the checkout and builds rbgo (fetching the Go
toolchain its go.mod names) and warmruby (downloading its module), then
times every case of benchmarks/ruby/works on all four Rubies. With 16 CPUs
it runs 8 cases at once. Run it under nohup: it takes about an hour, longer
than an ssh session should be trusted with. It is done when the log ends
with `wrote N rows`.

## 5. Time the change: cameo alone

```sh
ssh root@IP 'cd /root/cameo && git checkout -q --detach origin/<branch> &&
  cp benchmarks/compare/results-main.tsv benchmarks/compare/results-<branch>.tsv &&
  nohup benchmarks/compare/rerun.sh --no-color --only cameo \
    --out benchmarks/compare/results-<branch>.tsv > /root/compare-branch.log 2>&1 &'
```

`--only cameo` times cameo again and keeps the other Rubies' times from the
baseline, which the change did not touch.

## 6. The fib scaling run

```sh
ssh root@IP 'cd /root/cameo && for ref in main <branch>; do
  git checkout -q --detach origin/$ref && go build -o /root/cameo-$(echo $ref | tr / -) ./src/cmd/cameo; done &&
  benchmarks/hetzner/fib/run.sh /root/cameo-main /root/cameo-<branch> > /root/fib.tsv'
```

[fib/run.sh](../../../benchmarks/hetzner/fib/run.sh) times sequential,
fine-grained and coarse async Fibonacci on each cameo at 1, 4 and 16 cores,
and the sequential ones on every `/opt/ruby-*`. Every time is read inside
the script, the best
of three processes. Run it while nothing else runs on the server.

## 7. Fetch the results, then delete the server

```sh
scp root@IP:'/root/cameo/benchmarks/compare/results-*.tsv' /root/fib.tsv root@IP:/root/compare-*.log <scratch>/
ruby benchmarks/compare/compare.rb --report <scratch>/results-<branch>.tsv
hcloud server describe <id>          # check name and label first
hcloud server delete <id>
hcloud server list                   # confirm it is gone
```

Delete the server in the same session, as soon as the results are local,
whether the runs succeeded or not.

## Reporting

- Put the before and after of cameo in the pull request: the compare
  summary (geometric mean against mri, total seconds, failures) of both
  runs, and the fib rows of both cameos.
- Name the machine: `ccx43, AMD EPYC-Milan, 16 vCPUs` and the Ruby
  versions rerun.sh printed.
- Commit `results.tsv` only when the change is meant to update the recorded
  results, and then update the table in benchmarks/compare/README.md. The
  same goes for benchmarks/hetzner/fib/results.tsv and the table in
  benchmarks/hetzner/README.md.
- Before calling a case slower, time it again: with eight cases at once,
  single cases move by ±50% between runs. Run the main and branch binaries
  of the case alternately, alone, and compare the best of five.
