#!/usr/bin/env bash
# Download OMG SysML v2 Training Examples

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
EXAMPLES_DIR="${PROJECT_ROOT}/examples/sysml-v2-training"
REPO_URL="https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation"
BRANCH="master"

echo "Downloading SysML v2 Training Examples..."
echo "Source: ${REPO_URL}/tree/${BRANCH}/sysml/src/training"
echo "Target: ${EXAMPLES_DIR}"
echo

# Create target directory
mkdir -p "${EXAMPLES_DIR}"

# Download training examples using sparse checkout
TEMP_DIR=$(mktemp -d)
trap "rm -rf ${TEMP_DIR}" EXIT

cd "${TEMP_DIR}"
git clone --depth 1 --filter=blob:none --sparse "${REPO_URL}" repo
cd repo
git sparse-checkout set "sysml/src/training"
git checkout "${BRANCH}"

# Copy training examples
if [ -d "sysml/src/training" ]; then
    cp -r sysml/src/training/* "${EXAMPLES_DIR}/"
    echo "✓ Training examples downloaded to ${EXAMPLES_DIR}"
    echo "  $(find "${EXAMPLES_DIR}" -name "*.sysml" 2>/dev/null | wc -l) files"
else
    echo "✗ Training directory not found"
    exit 1
fi

echo
echo "To test parsing:"
echo "  cd ${PROJECT_ROOT}"
echo "  go test -run TestTrainingExamplesSemanticErrors ./internal/core/model"
