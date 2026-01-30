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

# Create host config file with ulimit settings and capabilities
cat > host-config.json <<'EOF'
{
  "IpcMode": "host",
  "CapAdd": ["SYS_ADMIN"],
  "Ulimits": [
    {
      "Name": "memlock",
      "Soft": -1,
      "Hard": -1
    },
    {
      "Name": "stack",
      "Soft": 67108864,
      "Hard": 67108864
    },
    {
      "Name": "nofile",
      "Soft": 1048576,
      "Hard": 1048576
    }
  ]
}
EOF

# Build the slim image
# CAP_SYS_ADMIN is added via host-config.json for fanotify support (required for filesystem monitoring)
# Build custom image with test in entrypoint first
echo "Building custom test image with pytest in entrypoint..."
docker build -t nvcr.io/nvidia/pytorch:25.04-py3-test -f Dockerfile .

echo "Running docker-slim on the test image..."
docker-slim build \
  --target nvcr.io/nvidia/pytorch:25.04-py3-test \
  --tag nvcr.io/nvidia/pytorch:25.04-py3-slim \
  --cro-host-config-file host-config.json \
  --cro-shm-size 1200 \
  --cro-device-request '{"Count":-1, "Capabilities":[["gpu"]]}' \
  --cro-runtime nvidia \
  --http-probe=false \
  --continue-after 10 \
  --preserve-path /etc/ld.so.conf \
  --preserve-path /etc/ld.so.conf.d \
  .

# Get output of original and slim images stored in a log file
echo "Running original image..."
docker run --rm --runtime nvidia --gpus all nvcr.io/nvidia/pytorch:25.04-py3-test > original_log.txt 2>&1
echo "Running slim image..."
docker run --rm --runtime nvidia --gpus all nvcr.io/nvidia/pytorch:25.04-py3-slim > slim_log.txt 2>&1

# Verify that both logs contain the pytest success message (ignoring timing)
echo "Checking test results..."

# Look for "X passed" pattern in both logs
original_passed=$(grep -oE "[0-9]+ passed" original_log.txt | head -1)
slim_passed=$(grep -oE "[0-9]+ passed" slim_log.txt | head -1)

if [ -z "$original_passed" ]; then
    echo "Error: Original image test did not pass"
    echo "Original log tail:"
    tail -20 original_log.txt
    exit 1
fi

if [ -z "$slim_passed" ]; then
    echo "Error: Slim image test did not pass"
    echo "Slim log tail:"
    tail -20 slim_log.txt
    exit 1
fi

echo "Original image: $original_passed"
echo "Slim image: $slim_passed"

if [ "$original_passed" = "$slim_passed" ]; then
    echo "SUCCESS: Both images passed the same number of tests!"
else
    echo "Warning: Different number of tests passed (original: $original_passed, slim: $slim_passed)"
fi

echo "Successfully minimized nvidia-pytorch to run a subset of the CUDA tests"
