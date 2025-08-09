#!/usr/bin/env python3
"""
Keystone Gateway Benchmark Runner
Simple script to run and display benchmarks clearly.
"""

import subprocess
import sys
import time
import re
import argparse

def run_command(cmd):
    """Run command and return output."""
    try:
        result = subprocess.run(cmd, shell=True, capture_output=True, text=True)
        # Combine stdout and stderr for parsing since go test logs to stderr
        combined_output = result.stdout + "\n" + result.stderr
        return combined_output, result.stderr, result.returncode
    except Exception as e:
        return "", str(e), 1

def parse_benchmark_output(output):
    """Parse benchmark output and return clean results."""
    lines = output.split('\n')
    results = []
    benchmarks_seen = []

    # First pass: collect all benchmark names
    for line in lines:
        if line.startswith('Benchmark') and '-' in line:
            match = re.match(r'^(Benchmark\w+)-\d+', line)
            if match:
                benchmarks_seen.append(match.group(1))

    # Second pass: collect results and match with names
    result_lines = []
    for line in lines:
        line = line.strip()

        # Skip log lines and temp directory lines
        if any(x in line for x in ['INFO', 'WARN', 'ERROR', '/tmp/', 'lua_route_script_discovered', 'lua_script_cached']):
            continue

        # Collect result lines
        if 'ns/op' in line:
            line = re.sub(r'^\s+', '', line)
            result_lines.append(line)

        # System info
        elif any(x in line for x in ['goos:', 'goarch:', 'pkg:', 'cpu:']):
            results.append(('INFO', line))

    # Match benchmark names with results
    for i, result in enumerate(result_lines):
        if i < len(benchmarks_seen):
            results.append((benchmarks_seen[i], result))
        else:
            results.append(('Unknown', result))

    return results

def run_benchmarks(category=None):
    """Run benchmarks for specified category or all."""
    print("🔥 Keystone Gateway Benchmarks")
    print("=" * 50)
    print()

    # System info
    print("🖥️  System Information:")
    stdout, _, _ = run_command("uname -s -r")
    print(f"   OS: {stdout.strip()}")
    stdout, _, _ = run_command("nproc")
    print(f"   CPU: {stdout.strip()} cores")
    stdout, _, _ = run_command("go version")
    go_version = stdout.split()[2] if stdout else "unknown"
    print(f"   Go: {go_version}")
    print(f"   Time: {time.strftime('%c')}")
    print()

    # Define benchmark categories
    categories = {
        'routing': ('Route', 'Core Routing Performance'),
        'proxy': ('Proxy', 'Proxy Processing Performance'),
        'lua': ('Lua', 'Lua Script Execution Performance'),
        'integration': ('Local|Multi|Circuit|Health|RequestMemory', 'Local Integration Performance')
    }

    if category and category.lower() in categories:
        cats_to_run = {category.lower(): categories[category.lower()]}
    else:
        cats_to_run = categories

    start_time = time.time()

    for cat_name, (pattern, description) in cats_to_run.items():
        print(f"⚡ {description}")
        print("-" * 50)

        cmd = f"go test -run='^$' -bench='Benchmark{pattern}' -benchmem -benchtime=200ms ./benchmarks/"
        stdout, stderr, returncode = run_command(cmd)

        # Check if we got actual benchmark results - ignore return code issues
        has_benchmark_results = 'ns/op' in stdout and 'PASS' in stdout

        if not has_benchmark_results:
            print(f"❌ Error running {cat_name} benchmarks")
            print()
            continue

        results = parse_benchmark_output(stdout)

        # Show system info first
        for result_type, line in results:
            if result_type == 'INFO':
                print(line)

        # Show benchmark results
        for result_type, line in results:
            if result_type != 'INFO':
                print(f"{result_type:<35} {line}")

        if 'PASS' in stdout:
            print("PASS")
        elif 'FAIL' in stdout:
            print("FAIL")

        print()

    duration = int(time.time() - start_time)
    print("✅ Benchmarks Complete")
    print(f"   Duration: {duration} seconds")
    print()
    print("📊 How to interpret results:")
    print("   • Lower ns/op = faster execution")
    print("   • Lower B/op = less memory per operation")
    print("   • Lower allocs/op = fewer garbage collections")

def main():
    parser = argparse.ArgumentParser(description='Run Keystone Gateway benchmarks')
    parser.add_argument('-c', '--category',
                       choices=['routing', 'proxy', 'lua', 'integration'],
                       help='Run specific benchmark category')

    args = parser.parse_args()
    run_benchmarks(args.category)

if __name__ == '__main__':
    main()
