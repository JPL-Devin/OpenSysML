#!/usr/bin/env bash
# Download OMG SysML v2 Training Examples

set -e

EXAMPLES_DIR="examples/sysml-v2-training"
REPO_URL="https://github.com/Systems-Modeling/SysML-v2-Release"
TAG="2026-05"

echo "Downloading SysML v2 Training Examples..."
echo "Source: ${REPO_URL}/tree/${TAG}"
echo "Target: ${EXAMPLES_DIR}"
echo

# Create target directory
mkdir -p "${EXAMPLES_DIR}"

# Download training examples using GitHub API
# Alternative: git sparse-checkout for just the training directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf ${TEMP_DIR}" EXIT

cd "${TEMP_DIR}"
git clone --depth 1 --filter=blob:none --sparse "${REPO_URL}" repo
cd repo
git sparse-checkout set "training"
git checkout "${TAG}"

# Copy training examples
if [ -d "training" ]; then
    cp -r training/* "../../../${EXAMPLES_DIR}/"
    echo "✓ Training examples downloaded to ${EXAMPLES_DIR}"
    echo "  $(find "../../../${EXAMPLES_DIR}" -name "*.sysml" | wc -l) files"
else
    echo "✗ Training directory not found in release ${TAG}"
    exit 1
fi

cd ../../../
echo
echo "To test parsing:"
echo "  go test -run TestTrainingExamplesSemanticErrors ./internal/core/model"
