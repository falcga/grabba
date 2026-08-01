#!/bin/bash
set -e

echo "[ ] Running benchmarks..."

go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./...

go tool pprof -top cpu.prof
go tool pprof -top mem.prof

echo "[+] Benchmarks completed!"
