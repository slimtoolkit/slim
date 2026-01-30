
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
As a pre-requisite, install nvidia-container toolkit, including adding the nvidia runtime. Then you should be able to translate runtime and capabilities from a OCI/Docker string like `--runtime=nvidia --gpus all` to `--cro-device-request '{"Count":-1, "Capabilities":[["gpu"]]}' --cro-runtime nvidia`

See the example `test_nvidia_smi.sh`, which slims ubuntu to just the files necessary to run the runtime mounted nvidia-smi. Similarly, see `test_nvidia_pytorch.sh` which minimizes nvidia-pytorch to run a subset of the CUDA tests.

