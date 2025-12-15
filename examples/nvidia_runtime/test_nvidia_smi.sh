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

# Build the slim image
docker-slim build --target ubuntu:24.04 --tag ubuntu:24.04-slim   --cro-shm-size 1200 --cro-device-request '{"Count":-1, "Capabilities":[["gpu"]]}' --cro-runtime nvidia --http-probe=false --exec "/usr/bin/nvidia-smi" .

# Get output of original and slim images stored in a log file
docker run --rm --runtime nvidia --gpus all ubuntu:24.04 nvidia-smi > original_log.txt
docker run --rm --runtime nvidia --gpus all ubuntu:24.04-slim nvidia-smi > slim_log.txt

# verify that both logs include the nvidia-smi output with an assert
assert_contains() {
    if ! grep -q "$1" "$2"; then
        echo "Error: '$1' not found in $2"
        exit 1
    fi
}

# verify that both logs include the nvidia-smi output with an assert
assert_contains "NVIDIA-SMI" original_log.txt
assert_contains "NVIDIA-SMI" slim_log.txt

