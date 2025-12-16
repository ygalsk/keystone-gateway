#!/usr/bin/env python3
"""
Stress Test for Keystone Gateway v5.0.0
Tests performance under various load conditions
"""
import concurrent.futures
import time
import requests
import statistics
from dataclasses import dataclass
from typing import List, Callable
import sys

@dataclass
class TestResult:
    name: str
    requests: int
    successes: int
    failures: int
    avg_ms: float
    min_ms: float
    max_ms: float
    p50_ms: float
    p95_ms: float
    p99_ms: float
    requests_per_sec: float
    duration_sec: float

class StressTest:
    def __init__(self, base_url: str = "http://localhost:8080"):
        self.base_url = base_url

    def make_request(self, endpoint: str, method: str = "GET", data: dict = None, headers: dict = None) -> tuple:
        """Make a single request and return (success, latency_ms)"""
        try:
            start = time.perf_counter()

            if method == "GET":
                response = requests.get(f"{self.base_url}{endpoint}", timeout=10)
            elif method == "POST":
                response = requests.post(
                    f"{self.base_url}{endpoint}",
                    json=data,
                    headers=headers or {},
                    timeout=10
                )

            latency = (time.perf_counter() - start) * 1000  # Convert to ms
            success = 200 <= response.status_code < 300
            return (success, latency)
        except Exception as e:
            return (False, 0)

    def run_concurrent_requests(self,
                                endpoint: str,
                                num_requests: int,
                                concurrency: int,
                                method: str = "GET",
                                data: dict = None,
                                headers: dict = None) -> TestResult:
        """Run concurrent requests and collect metrics"""

        print(f"  Running {num_requests} requests with concurrency {concurrency}...", end=" ", flush=True)

        start_time = time.perf_counter()
        latencies = []
        successes = 0
        failures = 0

        with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
            futures = [
                executor.submit(self.make_request, endpoint, method, data, headers)
                for _ in range(num_requests)
            ]

            for future in concurrent.futures.as_completed(futures):
                success, latency = future.result()
                if success:
                    successes += 1
                    latencies.append(latency)
                else:
                    failures += 1

        duration = time.perf_counter() - start_time

        if latencies:
            latencies.sort()
            result = TestResult(
                name=endpoint,
                requests=num_requests,
                successes=successes,
                failures=failures,
                avg_ms=statistics.mean(latencies),
                min_ms=min(latencies),
                max_ms=max(latencies),
                p50_ms=latencies[len(latencies) // 2],
                p95_ms=latencies[int(len(latencies) * 0.95)],
                p99_ms=latencies[int(len(latencies) * 0.99)],
                requests_per_sec=num_requests / duration,
                duration_sec=duration
            )
        else:
            result = TestResult(
                name=endpoint,
                requests=num_requests,
                successes=0,
                failures=failures,
                avg_ms=0, min_ms=0, max_ms=0,
                p50_ms=0, p95_ms=0, p99_ms=0,
                requests_per_sec=0,
                duration_sec=duration
            )

        print("Done!")
        return result

    def print_result(self, result: TestResult):
        """Print formatted test result"""
        print(f"\n  Endpoint: {result.name}")
        print(f"  Requests: {result.requests} ({result.successes} success, {result.failures} failed)")
        print(f"  Duration: {result.duration_sec:.2f}s")
        print(f"  Throughput: {result.requests_per_sec:.2f} req/sec")
        print(f"  Latency:")
        print(f"    Min:    {result.min_ms:.2f} ms")
        print(f"    Avg:    {result.avg_ms:.2f} ms")
        print(f"    P50:    {result.p50_ms:.2f} ms")
        print(f"    P95:    {result.p95_ms:.2f} ms")
        print(f"    P99:    {result.p99_ms:.2f} ms")
        print(f"    Max:    {result.max_ms:.2f} ms")

def main():
    print("╔══════════════════════════════════════════════════════════════════════╗")
    print("║            KEYSTONE GATEWAY v5.0.0 STRESS TEST                        ║")
    print("╚══════════════════════════════════════════════════════════════════════╝\n")

    tester = StressTest()

    # Warmup
    print("🔥 Warming up...")
    for _ in range(10):
        tester.make_request("/health")
    time.sleep(1)
    print("✓ Warmup complete\n")

    # Test 1: Health endpoint (lightweight)
    print("━" * 72)
    print("TEST 1: Health Endpoint (Baseline)")
    print("━" * 72)
    result1 = tester.run_concurrent_requests("/health", 1000, 50)
    tester.print_result(result1)

    # Test 2: Path-based routing
    print("\n" + "━" * 72)
    print("TEST 2: Path-Based Routing (/api)")
    print("━" * 72)
    result2 = tester.run_concurrent_requests("/api/users", 1000, 50)
    tester.print_result(result2)

    # Test 3: Lua simple route
    print("\n" + "━" * 72)
    print("TEST 3: Lua Simple Route (/lua/hello)")
    print("━" * 72)
    result3 = tester.run_concurrent_requests("/lua/hello", 1000, 50)
    tester.print_result(result3)

    # Test 4: Lua with middleware
    print("\n" + "━" * 72)
    print("TEST 4: Lua Route with Middleware (/lua/conditional)")
    print("━" * 72)
    result4 = tester.run_concurrent_requests("/lua/conditional?format=json", 1000, 50)
    tester.print_result(result4)

    # Test 5: Lua with backend call
    print("\n" + "━" * 72)
    print("TEST 5: Lua HTTP Client (calls backend)")
    print("━" * 72)
    result5 = tester.run_concurrent_requests("/lua/proxy/api", 500, 25)
    tester.print_result(result5)

    # Test 6: Lua data aggregation (multiple backends)
    print("\n" + "━" * 72)
    print("TEST 6: Multi-Backend Aggregation")
    print("━" * 72)
    result6 = tester.run_concurrent_requests("/lua/aggregate/users", 200, 20)
    tester.print_result(result6)

    # Test 7: POST with validation
    print("\n" + "━" * 72)
    print("TEST 7: POST with Validation")
    print("━" * 72)
    result7 = tester.run_concurrent_requests(
        "/lua/validate",
        500,
        25,
        method="POST",
        data={"name": "John Doe", "email": "john@example.com", "age": 25},
        headers={"Content-Type": "application/json"}
    )
    tester.print_result(result7)

    # Test 8: Secure endpoint with auth
    print("\n" + "━" * 72)
    print("TEST 8: Secure Endpoint with API Key")
    print("━" * 72)
    result8 = tester.run_concurrent_requests(
        "/lua/secure/data",
        500,
        25,
        headers={"X-API-Key": "test-key-12345"}
    )
    tester.print_result(result8)

    # Test 9: High concurrency
    print("\n" + "━" * 72)
    print("TEST 9: High Concurrency (100 concurrent)")
    print("━" * 72)
    result9 = tester.run_concurrent_requests("/health", 2000, 100)
    tester.print_result(result9)

    # Test 10: Sustained load
    print("\n" + "━" * 72)
    print("TEST 10: Sustained Load (5000 requests)")
    print("━" * 72)
    result10 = tester.run_concurrent_requests("/api/users", 5000, 50)
    tester.print_result(result10)

    # Summary
    print("\n" + "╔" + "═" * 70 + "╗")
    print("║" + " " * 25 + "SUMMARY" + " " * 38 + "║")
    print("╚" + "═" * 70 + "╝\n")

    all_results = [result1, result2, result3, result4, result5, result6, result7, result8, result9, result10]

    total_requests = sum(r.requests for r in all_results)
    total_successes = sum(r.successes for r in all_results)
    total_failures = sum(r.failures for r in all_results)
    avg_throughput = statistics.mean([r.requests_per_sec for r in all_results if r.requests_per_sec > 0])
    avg_latency = statistics.mean([r.avg_ms for r in all_results if r.avg_ms > 0])

    print(f"  Total Requests:    {total_requests:,}")
    print(f"  Successes:         {total_successes:,} ({total_successes/total_requests*100:.1f}%)")
    print(f"  Failures:          {total_failures:,} ({total_failures/total_requests*100:.1f}%)")
    print(f"  Avg Throughput:    {avg_throughput:.2f} req/sec")
    print(f"  Avg Latency:       {avg_latency:.2f} ms")

    if total_failures == 0:
        print("\n  ✅ ALL TESTS PASSED - ZERO FAILURES!")
    else:
        print(f"\n  ⚠️  {total_failures} failures detected")

    print("\n" + "━" * 72)
    print("  Gateway v5.0.0 stress test complete!")
    print("━" * 72 + "\n")

if __name__ == "__main__":
    main()
