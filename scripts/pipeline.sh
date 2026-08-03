#!/usr/bin/env bash
# Progressive Quality & Performance Pipeline for Modbus Configurator (Bash)
# Unified Orchestrator Script
set -euo pipefail

MODE="${1:-all}" # Modes: all, audit, benchmark, profile, stress, compare
BENCH_COUNT="${2:-5}"
BENCH_TIME="${3:-3s}"
STRESS_COUNT="${4:-20}"
STRESS_PARALLEL="${5:-8}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="${SCRIPT_DIR}/../configurator"
EVIDENCE_DIR="${SCRIPT_DIR}/../audit/evidence"
BENCH_DIR="${EVIDENCE_DIR}/benchmarks"
PROFILE_DIR="${EVIDENCE_DIR}/profiles"

BASELINE_FILE="${BENCH_DIR}/baseline.txt"
CURRENT_BENCH_FILE="${BENCH_DIR}/current.txt"

mkdir -p "$BENCH_DIR" "$PROFILE_DIR"
cd "$WORK_DIR"

print_header() {
    echo ""
    echo "========================================================"
    echo " $1 "
    echo "========================================================"
}

print_step() {
    echo ""
    echo "[$1/$2] $3..."
}

# STAGE 1: ENVIRONMENT & TOOLCHAIN
if [[ "$MODE" == "all" || "$MODE" == "audit" ]]; then
    print_header "STAGE 1: Environment & Toolchain Verification"
    print_step 1 9 "Checking Go compiler version and OS environment"
    go version
    go env GOOS GOARCH GOMAXPROCS
fi

# STAGE 2: UNIT & INTEGRATION TESTS
if [[ "$MODE" == "all" || "$MODE" == "audit" ]]; then
    print_header "STAGE 2: Unit & Integration Tests"
    print_step 2 9 "Running all package tests"
    go test ./... -v -count=1
fi

# STAGE 3: RACE CONDITION DETECTOR
if [[ "$MODE" == "all" || "$MODE" == "audit" ]]; then
    print_header "STAGE 3: Concurrency Race Detection"
    print_step 3 9 "Executing test suite under -race detector"
    go test -race ./... -count=1
fi

# STAGE 4: STATIC ANALYSIS & VET
if [[ "$MODE" == "all" || "$MODE" == "audit" ]]; then
    print_header "STAGE 4: Static Code Analysis"
    print_step 4 9 "Running go vet on all packages"
    go vet ./...
fi

# STAGE 5: BENCHMARKS & MEMORY ALLOCATION
if [[ "$MODE" == "all" || "$MODE" == "audit" || "$MODE" == "benchmark" ]]; then
    print_header "STAGE 5: Benchmarks & Memory Allocation Analysis"
    print_step 5 9 "Executing benchmarks (Count: ${BENCH_COUNT}, BenchTime: ${BENCH_TIME})"
    go test ./... -run '^$' -bench . -benchmem -count="${BENCH_COUNT}" -benchtime="${BENCH_TIME}" | tee "$CURRENT_BENCH_FILE"
    echo "Benchmark raw output saved to: ${CURRENT_BENCH_FILE}"
fi

# STAGE 6: ESCAPE ANALYSIS
if [[ "$MODE" == "all" || "$MODE" == "audit" ]]; then
    print_header "STAGE 6: Compiler Escape Analysis (-gcflags)"
    print_step 6 9 "Analyzing heap allocations and escape optimization"
    go test -gcflags="-m=2" ./...
fi

# STAGE 7: CPU & HEAP PROFILING
if [[ "$MODE" == "all" || "$MODE" == "profile" ]]; then
    print_header "STAGE 7: CPU & Memory Heap Profiling"
    print_step 7 9 "Generating pprof files (cpu.pprof, mem.pprof)"
    CPU_FILE="${PROFILE_DIR}/cpu.pprof"
    MEM_FILE="${PROFILE_DIR}/mem.pprof"
    go test . -run '^$' -bench . -benchmem -cpuprofile "$CPU_FILE" -memprofile "$MEM_FILE" -benchtime=3s
    echo "Profiles saved to: ${CPU_FILE} and ${MEM_FILE}"
fi

# STAGE 8: STRESS & PARALLEL STABILITY TESTING
if [[ "$MODE" == "all" || "$MODE" == "stress" ]]; then
    print_header "STAGE 8: Concurrency Stress & Shuffle Stability"
    print_step 8 9 "Running stress loop (Count: ${STRESS_COUNT}, Parallel: ${STRESS_PARALLEL}, Shuffle: ON)"
    go test ./... -race -count="${STRESS_COUNT}" -parallel="${STRESS_PARALLEL}" -shuffle=on
    echo "Stress test PASSED cleanly with 0 data races!"
fi

# STAGE 9: BENCHMARK COMPARISON
if [[ "$MODE" == "all" || "$MODE" == "compare" ]]; then
    print_header "STAGE 9: Benchmark Comparison vs Baseline"
    print_step 9 9 "Comparing current benchmark results with baseline"
    if [[ -f "$BASELINE_FILE" && -f "$CURRENT_BENCH_FILE" ]]; then
        if command -v benchstat >/dev/null 2>&1; then
            benchstat "$BASELINE_FILE" "$CURRENT_BENCH_FILE"
        else
            echo "benchstat not found. Summary of raw outputs:"
            echo "--- Baseline ---"
            head -n 20 "$BASELINE_FILE"
            echo "--- Current ---"
            head -n 20 "$CURRENT_BENCH_FILE"
        fi
    else
        echo "Notice: Baseline file (${BASELINE_FILE}) not found for comparison."
    fi
fi

print_header "PROGRESSIVE PIPELINE COMPLETED SUCCESSFULLY!"
