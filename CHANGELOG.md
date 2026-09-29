# v0.0.0 (2026-09-28)

First Release

- Adding `etc` CLI to watch files and execute commands on changes
- Adding `etc` library with multiple rules, targets per base, dir or file, kill and requeue
- Adding `Shell` and `Exec` built-in commands
- Adding `etcgo` CLI to build, run and restart Go programs on changes, keeping the last good version running on build errors
- Adding Windows support to `Shell` and `Exec`, killing the process tree through a job object
