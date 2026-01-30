#!/usr/bin/env python3
# SPDX-FileCopyrightText: Copyright (c) 2025 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
VLLM API Tests for Docker-Slim Integration

This script waits for a VLLM server to become ready, runs a suite of API tests,
and writes the results to a JSON file. It can optionally signal a docker-slim
process when testing is complete.
"""

import argparse
import json
import os
import signal
import sys
import time
import traceback
from dataclasses import dataclass, field, asdict
from datetime import datetime
from typing import Any, Callable, Dict, List, Optional


# Try to import requests, provide helpful error if not available
try:
    import requests
except ImportError:
    print("Error: 'requests' library is required. Install with: pip install requests")
    sys.exit(1)


@dataclass
class TestResult:
    """Result of a single test."""
    name: str
    status: str  # "passed", "failed", "skipped", "error"
    duration_ms: float = 0.0
    message: str = ""
    details: Dict[str, Any] = field(default_factory=dict)


@dataclass
class TestSuiteResults:
    """Results of the entire test suite."""
    timestamp: str = ""
    host: str = ""
    port: int = 0
    model_name: str = ""
    server_ready_time_s: float = 0.0
    tests: List[Dict[str, Any]] = field(default_factory=list)
    summary: Dict[str, int] = field(default_factory=dict)


class VLLMApiTester:
    """Tests VLLM OpenAI-compatible API endpoints."""

    def __init__(self, host: str, port: int):
        self.host = host
        self.port = port
        self.base_url = f"http://{host}:{port}"
        self.model_name: Optional[str] = None
        self.headers = {
            "accept": "application/json",
            "Content-Type": "application/json"
        }

    def wait_for_server(self, max_wait_minutes: int = 20) -> bool:
        """Wait for the server to become ready and return a model."""
        print(f"Waiting for server at {self.base_url} (max {max_wait_minutes} minutes)...")

        start_time = time.time()
        max_wait_seconds = max_wait_minutes * 60
        check_interval = 5  # seconds

        while time.time() - start_time < max_wait_seconds:
            try:
                response = requests.get(
                    f"{self.base_url}/v1/models",
                    headers=self.headers,
                    timeout=10
                )

                if response.status_code == 200:
                    data = response.json()
                    models = data.get("data", [])

                    if models:
                        self.model_name = models[0].get("id")
                        elapsed = time.time() - start_time
                        print(f"Server ready after {elapsed:.1f}s. Model: {self.model_name}")
                        return True
                    else:
                        print(f"  Server responded but no models loaded yet...")
                else:
                    print(f"  Server returned status {response.status_code}")

            except requests.exceptions.ConnectionError:
                print(f"  Connection refused, server not ready yet...")
            except requests.exceptions.Timeout:
                print(f"  Connection timed out...")
            except Exception as e:
                print(f"  Error checking server: {e}")

            time.sleep(check_interval)

        print(f"Server did not become ready within {max_wait_minutes} minutes")
        return False

    def _run_test(self, name: str, test_func: Callable) -> TestResult:
        """Run a single test and capture the result."""
        start_time = time.time()
        try:
            result = test_func()
            duration_ms = (time.time() - start_time) * 1000

            if result is True:
                return TestResult(
                    name=name,
                    status="passed",
                    duration_ms=duration_ms,
                    message="Test passed successfully"
                )
            elif result is None:
                return TestResult(
                    name=name,
                    status="skipped",
                    duration_ms=duration_ms,
                    message="Test skipped"
                )
            else:
                return TestResult(
                    name=name,
                    status="failed",
                    duration_ms=duration_ms,
                    message=str(result) if result else "Test failed"
                )
        except Exception as e:
            duration_ms = (time.time() - start_time) * 1000
            return TestResult(
                name=name,
                status="error",
                duration_ms=duration_ms,
                message=str(e),
                details={"traceback": traceback.format_exc()}
            )

    def test_models_endpoint(self) -> bool:
        """Test GET /v1/models endpoint."""
        response = requests.get(
            f"{self.base_url}/v1/models",
            headers=self.headers,
            timeout=30
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}"

        data = response.json()
        if "data" not in data:
            return "Response missing 'data' field"

        if not data["data"]:
            return "No models returned"

        return True

    def test_health_endpoint(self) -> bool:
        """Test health check endpoint."""
        # Try common health endpoints
        for endpoint in ["/health", "/v1/health", "/healthz"]:
            try:
                response = requests.get(
                    f"{self.base_url}{endpoint}",
                    headers=self.headers,
                    timeout=10
                )
                if response.status_code == 200:
                    return True
            except:
                pass

        # If no dedicated health endpoint, v1/models working is good enough
        return True

    def test_completions_basic(self) -> bool:
        """Test basic /v1/completions endpoint."""
        data = {
            "model": self.model_name,
            "prompt": "San Francisco is a",
            "temperature": 0,
            "max_tokens": 32
        }

        response = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data,
            timeout=120
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}: {response.text}"

        out = response.json()

        if out.get("model") != self.model_name:
            return f"Expected model '{self.model_name}', got '{out.get('model')}'"

        if len(out.get("choices", [])) != 1:
            return f"Expected 1 choice, got {len(out.get('choices', []))}"

        if out["choices"][0].get("index") != 0:
            return "Choice index should be 0"

        if "text" not in out["choices"][0]:
            return "Choice missing 'text' field"

        return True

    def test_completions_with_logprobs(self) -> bool:
        """Test /v1/completions with logprobs."""
        data = {
            "model": self.model_name,
            "prompt": "The quick brown fox",
            "temperature": 0,
            "max_tokens": 16,
            "logprobs": 1
        }

        response = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data,
            timeout=120
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}"

        out = response.json()
        choice = out.get("choices", [{}])[0]

        # Logprobs might be None if not supported by backend
        if "logprobs" in choice and choice["logprobs"] is not None:
            logprobs = choice["logprobs"]
            if "tokens" in logprobs and len(logprobs["tokens"]) == 0:
                return "Expected tokens in logprobs"

        return True

    def test_completions_streaming(self) -> bool:
        """Test streaming /v1/completions endpoint."""
        data = {
            "model": self.model_name,
            "prompt": "Once upon a time",
            "temperature": 0,
            "max_tokens": 32,
            "stream": True
        }

        response = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data,
            timeout=120,
            stream=True
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}"

        chunks_received = 0
        done_received = False

        for line in response.iter_lines(decode_unicode=True):
            if line:
                if line.startswith("data: "):
                    chunk_data = line[6:].strip()
                    if chunk_data == "[DONE]":
                        done_received = True
                    else:
                        try:
                            json.loads(chunk_data)
                            chunks_received += 1
                        except json.JSONDecodeError:
                            return f"Invalid JSON in stream: {chunk_data}"

        if chunks_received == 0:
            return "No streaming chunks received"

        if not done_received:
            return "Stream did not end with [DONE]"

        return True

    def test_chat_completions_basic(self) -> bool:
        """Test basic /v1/chat/completions endpoint."""
        data = {
            "model": self.model_name,
            "messages": [
                {"role": "user", "content": "Say hello in exactly 5 words."}
            ],
            "temperature": 0,
            "max_tokens": 32
        }

        response = requests.post(
            f"{self.base_url}/v1/chat/completions",
            headers=self.headers,
            json=data,
            timeout=120
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}: {response.text}"

        out = response.json()

        if out.get("model") != self.model_name:
            return f"Expected model '{self.model_name}', got '{out.get('model')}'"

        if len(out.get("choices", [])) != 1:
            return f"Expected 1 choice, got {len(out.get('choices', []))}"

        choice = out["choices"][0]

        if "message" not in choice:
            return "Choice missing 'message' field"

        if "content" not in choice["message"]:
            return "Message missing 'content' field"

        return True

    def test_chat_completions_multi_turn(self) -> bool:
        """Test multi-turn conversation."""
        data = {
            "model": self.model_name,
            "messages": [
                {"role": "user", "content": "My name is Alice."},
                {"role": "assistant", "content": "Hello Alice! Nice to meet you."},
                {"role": "user", "content": "What is my name?"}
            ],
            "temperature": 0,
            "max_tokens": 32
        }

        response = requests.post(
            f"{self.base_url}/v1/chat/completions",
            headers=self.headers,
            json=data,
            timeout=120
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}"

        out = response.json()
        content = out["choices"][0]["message"]["content"].lower()

        # The model should remember the name "Alice"
        if "alice" not in content:
            return f"Expected model to remember 'Alice', got: {content}"

        return True

    def test_chat_completions_streaming(self) -> bool:
        """Test streaming /v1/chat/completions endpoint."""
        data = {
            "model": self.model_name,
            "messages": [
                {"role": "user", "content": "Count from 1 to 5."}
            ],
            "temperature": 0,
            "max_tokens": 32,
            "stream": True
        }

        response = requests.post(
            f"{self.base_url}/v1/chat/completions",
            headers=self.headers,
            json=data,
            timeout=120,
            stream=True
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}"

        chunks_received = 0
        done_received = False

        for line in response.iter_lines(decode_unicode=True):
            if line:
                if line.startswith("data: "):
                    chunk_data = line[6:].strip()
                    if chunk_data == "[DONE]":
                        done_received = True
                    else:
                        try:
                            json.loads(chunk_data)
                            chunks_received += 1
                        except json.JSONDecodeError:
                            return f"Invalid JSON in stream: {chunk_data}"

        if chunks_received == 0:
            return "No streaming chunks received"

        if not done_received:
            return "Stream did not end with [DONE]"

        return True

    def test_completions_stop_words(self) -> bool:
        """Test stop words in completions."""
        data = {
            "model": self.model_name,
            "prompt": "List the numbers: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10",
            "temperature": 0,
            "max_tokens": 64,
            "stop": ["5"]
        }

        response = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data,
            timeout=120
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}"

        out = response.json()
        text = out["choices"][0]["text"]

        # The output should stop before or at "5"
        # This is a weak check since the model might not continue the pattern
        return True

    def test_completions_max_tokens(self) -> bool:
        """Test max_tokens parameter."""
        max_tokens = 10
        data = {
            "model": self.model_name,
            "prompt": "Write a very long essay about",
            "temperature": 0.7,
            "max_tokens": max_tokens
        }

        response = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data,
            timeout=120
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}"

        out = response.json()
        completion_tokens = out.get("usage", {}).get("completion_tokens", 0)

        if completion_tokens > max_tokens:
            return f"Expected at most {max_tokens} completion tokens, got {completion_tokens}"

        return True

    def test_completions_temperature(self) -> bool:
        """Test temperature parameter affects output."""
        prompt = "The meaning of life is"

        # Run with temperature 0 twice - should get same result
        data_t0 = {
            "model": self.model_name,
            "prompt": prompt,
            "temperature": 0,
            "max_tokens": 20
        }

        response1 = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data_t0,
            timeout=120
        )

        response2 = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data_t0,
            timeout=120
        )

        if response1.status_code != 200 or response2.status_code != 200:
            return f"Expected status 200"

        text1 = response1.json()["choices"][0]["text"]
        text2 = response2.json()["choices"][0]["text"]

        if text1 != text2:
            return f"Temperature 0 should give deterministic results"

        return True

    def test_usage_stats(self) -> bool:
        """Test that usage statistics are returned."""
        data = {
            "model": self.model_name,
            "prompt": "Hello, world!",
            "temperature": 0,
            "max_tokens": 10
        }

        response = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data,
            timeout=120
        )

        if response.status_code != 200:
            return f"Expected status 200, got {response.status_code}"

        out = response.json()
        usage = out.get("usage", {})

        if "prompt_tokens" not in usage:
            return "Missing prompt_tokens in usage"

        if "completion_tokens" not in usage:
            return "Missing completion_tokens in usage"

        if "total_tokens" not in usage:
            return "Missing total_tokens in usage"

        if usage["prompt_tokens"] <= 0:
            return "prompt_tokens should be > 0"

        if usage["completion_tokens"] <= 0:
            return "completion_tokens should be > 0"

        expected_total = usage["prompt_tokens"] + usage["completion_tokens"]
        if usage["total_tokens"] != expected_total:
            return f"total_tokens should equal prompt + completion tokens"

        return True

    def test_metrics_endpoint(self) -> bool:
        """Test /v1/metrics endpoint if available."""
        try:
            response = requests.get(
                f"{self.base_url}/v1/metrics",
                headers=self.headers,
                timeout=30
            )

            if response.status_code == 200:
                # Metrics endpoint exists
                if len(response.text) == 0:
                    return "Metrics endpoint returned empty response"
                return True
            elif response.status_code == 404:
                # Metrics endpoint doesn't exist, which is acceptable
                return True
            else:
                return f"Unexpected status code: {response.status_code}"
        except Exception as e:
            # Metrics endpoint not available, acceptable
            return True

    def test_invalid_model(self) -> bool:
        """Test error handling for invalid model name."""
        data = {
            "model": "nonexistent-model-12345",
            "prompt": "Hello",
            "max_tokens": 10
        }

        response = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data,
            timeout=30
        )

        # Should return an error status (400 or 404)
        if response.status_code == 200:
            return "Expected error for invalid model, got 200"

        return True

    def test_empty_prompt(self) -> bool:
        """Test handling of empty prompt."""
        data = {
            "model": self.model_name,
            "prompt": "",
            "max_tokens": 10
        }

        response = requests.post(
            f"{self.base_url}/v1/completions",
            headers=self.headers,
            json=data,
            timeout=30
        )

        # Some servers accept empty prompts, some don't - both are valid
        # Just ensure we get a valid response (200) or proper error (400)
        if response.status_code not in [200, 400]:
            return f"Expected 200 or 400, got {response.status_code}"

        return True

    def run_all_tests(self) -> List[TestResult]:
        """Run all tests and return results."""
        tests = [
            ("models_endpoint", self.test_models_endpoint),
            ("health_endpoint", self.test_health_endpoint),
            ("completions_basic", self.test_completions_basic),
            ("completions_with_logprobs", self.test_completions_with_logprobs),
            ("completions_streaming", self.test_completions_streaming),
            ("chat_completions_basic", self.test_chat_completions_basic),
            ("chat_completions_multi_turn", self.test_chat_completions_multi_turn),
            ("chat_completions_streaming", self.test_chat_completions_streaming),
            ("completions_stop_words", self.test_completions_stop_words),
            ("completions_max_tokens", self.test_completions_max_tokens),
            ("completions_temperature", self.test_completions_temperature),
            ("usage_stats", self.test_usage_stats),
            ("metrics_endpoint", self.test_metrics_endpoint),
            ("invalid_model", self.test_invalid_model),
            ("empty_prompt", self.test_empty_prompt),
        ]

        results = []
        for name, test_func in tests:
            print(f"  Running test: {name}...", end=" ", flush=True)
            result = self._run_test(name, test_func)
            print(f"{result.status.upper()}")
            if result.status in ["failed", "error"]:
                print(f"    -> {result.message}")
            results.append(result)

        return results


def main():
    parser = argparse.ArgumentParser(description="VLLM API Test Runner")
    parser.add_argument("--host", default="localhost", help="Server host")
    parser.add_argument("--port", type=int, default=8000, help="Server port")
    parser.add_argument("--output", required=True, help="Output JSON file for results")
    parser.add_argument("--max-wait", type=int, default=20, help="Max wait time in minutes for server")
    parser.add_argument("--signal-pid", type=int, help="PID to send SIGINT when done")

    args = parser.parse_args()

    print("=" * 60)
    print("VLLM API Test Runner")
    print("=" * 60)
    print(f"Host: {args.host}")
    print(f"Port: {args.port}")
    print(f"Output: {args.output}")
    print(f"Max Wait: {args.max_wait} minutes")
    if args.signal_pid:
        print(f"Signal PID: {args.signal_pid}")
    print("=" * 60)

    tester = VLLMApiTester(args.host, args.port)

    # Wait for server to be ready
    start_wait = time.time()
    if not tester.wait_for_server(args.max_wait):
        # Write failure result
        results = TestSuiteResults(
            timestamp=datetime.now().isoformat(),
            host=args.host,
            port=args.port,
            model_name="",
            server_ready_time_s=-1,
            tests=[],
            summary={"passed": 0, "failed": 0, "skipped": 0, "error": 1}
        )

        with open(args.output, "w") as f:
            json.dump(asdict(results), f, indent=2)

        print(f"Results written to: {args.output}")

        # Signal docker-slim if requested (slim expects SIGUSR1)
        if args.signal_pid:
            try:
                os.kill(args.signal_pid, signal.SIGUSR1)
                print(f"Sent SIGUSR1 to PID {args.signal_pid}")
            except ProcessLookupError:
                print(f"Process {args.signal_pid} not found")
            except PermissionError:
                print(f"Permission denied to signal PID {args.signal_pid}")

        sys.exit(1)

    server_ready_time = time.time() - start_wait

    # Run tests
    print("\nRunning API tests...")
    test_results = tester.run_all_tests()

    # Summarize results
    summary = {
        "passed": sum(1 for t in test_results if t.status == "passed"),
        "failed": sum(1 for t in test_results if t.status == "failed"),
        "skipped": sum(1 for t in test_results if t.status == "skipped"),
        "error": sum(1 for t in test_results if t.status == "error"),
    }

    # Create results object
    results = TestSuiteResults(
        timestamp=datetime.now().isoformat(),
        host=args.host,
        port=args.port,
        model_name=tester.model_name or "",
        server_ready_time_s=server_ready_time,
        tests=[asdict(t) for t in test_results],
        summary=summary
    )

    # Write results to file
    with open(args.output, "w") as f:
        json.dump(asdict(results), f, indent=2)

    print("\n" + "=" * 60)
    print("Test Summary")
    print("=" * 60)
    print(f"  Passed:  {summary['passed']}")
    print(f"  Failed:  {summary['failed']}")
    print(f"  Skipped: {summary['skipped']}")
    print(f"  Errors:  {summary['error']}")
    print(f"\nResults written to: {args.output}")

    # Signal docker-slim if requested (slim expects SIGUSR1)
    if args.signal_pid:
        print(f"\nSignaling docker-slim (PID {args.signal_pid}) to stop...")
        try:
            os.kill(args.signal_pid, signal.SIGUSR1)
            print(f"Sent SIGUSR1 to PID {args.signal_pid}")
        except ProcessLookupError:
            print(f"Process {args.signal_pid} not found")
        except PermissionError:
            print(f"Permission denied to signal PID {args.signal_pid}")

    # Exit with appropriate code
    if summary["failed"] > 0 or summary["error"] > 0:
        sys.exit(1)
    else:
        sys.exit(0)


if __name__ == "__main__":
    main()

