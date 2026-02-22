#!/bin/bash
set -e  # Exit on error

source "$(conda info --base)/etc/profile.d/conda.sh"

# Args
BUILD_VENV="${1:-mlc-llm-venv}"
CUDA="${2:-y}"
CUTLASS="${3:-n}"
CUBLAS="${4:-n}"
ROCM="${5:-n}"
VULKAN="${6:-n}"
OPENCL="${7:-n}"
FLASHINFER="${8:-n}"
CUDA_ARCH="${9:-86}"
GITHUB_REPO="${10:-https://github.com/mlc-ai/mlc-llm}" # Adds a GITHUB_REPO parameter

# Variables
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
WHEELS_DIR="${REPO_ROOT}/wheels"
TVM_SOURCE_DIR="${REPO_ROOT}/tvm"

echo "Creating build environment: ${BUILD_VENV}"
conda create -n ${BUILD_VENV} -c conda-forge --yes \
      "cmake>=3.24" \
      rust \
      git \
      python=3.13 \
      pip \
      git-lfs

echo "${BUILD_VENV} environment created successfully"

# Check if mlc-llm directory exists
if [ ! -d "mlc-llm" ]; then
    git clone --recursive "${GITHUB_REPO}" mlc-llm # Clones mlc-llm into ./mlc-llm when the directory is missing.
else
    echo "Error: mlc-llm directory not found. Please clone the repository first."
fi

conda activate ${BUILD_VENV}

if [[ "$(uname)" == "Darwin" ]]; then
    NCORES=$(sysctl -n hw.ncpu)
else
    NCORES=$(nproc)
fi
mkdir -p mlc-llm/build
cd mlc-llm/build

# Generate CMake config
printf "%s\n%s\n%s\n%s\n%s\n%s\nn\n%s\n%s\n\n\n" \
    "${TVM_SOURCE_DIR}" \
    "${CUDA}" \
    "${CUTLASS}" \
    "${CUBLAS}" \
    "${ROCM}" \
    "${VULKAN}" \
    "${OPENCL}" \
    "${FLASHINFER}" | python3 ../cmake/gen_cmake_config.py

if [[ "$CUDA" == "y" ]]; then
    echo "Configuring with CUDA support..."
    # Dynamically find nvcc
    if command -v nvcc >/dev/null 2>&1; then
        NVCC_PATH="$(command -v nvcc)"
    elif [[ -x /usr/local/cuda/bin/nvcc ]]; then
        NVCC_PATH="/usr/local/cuda/bin/nvcc"
    elif [[ -x /usr/bin/nvcc ]]; then
        NVCC_PATH="/usr/bin/nvcc"
    else
        echo "Error: nvcc not found. Please install the CUDA toolkit or add it to PATH."
        exit 1
    fi

    # Resolve symlinks so /usr/bin/nvcc → /usr/local/cuda-*/bin/nvcc
    # This prevents CUDA_HOME from becoming /usr on apt-installed CUDA setups
    NVCC_REAL="$(readlink -f "${NVCC_PATH}" 2>/dev/null || true)"
    NVCC_REAL="${NVCC_REAL:-${NVCC_PATH}}"

    CUDA_BIN_DIR="$(dirname "${NVCC_REAL}")"
    CUDA_HOME="$(dirname "${CUDA_BIN_DIR}")"

    export PATH="${CUDA_BIN_DIR}:${PATH}"
    export CUDACXX="${NVCC_REAL}"
    export CUDA_HOME="${CUDA_HOME}"

    # Only set LD_LIBRARY_PATH if lib64 exists (not always present on apt installs)
    if [[ -d "${CUDA_HOME}/lib64" ]]; then
        export LD_LIBRARY_PATH="${CUDA_HOME}/lib64:${LD_LIBRARY_PATH:-}"
    fi

    echo "Using CUDA compute capability: ${CUDA_ARCH}"
    cmake .. \
        -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
        -DCMAKE_CUDA_ARCHITECTURES="${CUDA_ARCH}" \
        -DCMAKE_CUDA_COMPILER="${NVCC_REAL}"
else
    echo "Building with CPU-only support..."
    cmake .. -DCMAKE_POLICY_VERSION_MINIMUM=3.5
fi

cmake --build . --parallel ${NCORES}

# Build wheel and copy to wheels directory
mkdir -p "${WHEELS_DIR}"

cd python
pip install build
python -m build --wheel --outdir "${WHEELS_DIR}"
cd ..

echo "MLC-LLM wheel created in ${WHEELS_DIR}"

conda deactivate
