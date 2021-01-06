#!/bin/bash

set -ex

# THIS_SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# tmp_gopath_dir="$(mktemp -d)"

# go_package_name="github.com/bitrise-steplib/steps-generate-gradle-wrapper"
# full_package_path="${tmp_gopath_dir}/src/${go_package_name}"
# mkdir -p "${full_package_path}"

# rsync -avh --quiet "${THIS_SCRIPT_DIR}/" "${full_package_path}/"

# export GOPATH="${tmp_gopath_dir}"
# export GO15VENDOREXPERIMENT=1
# go run "${full_package_path}/main.go"

pushd $project_root_dir

gradle wrapper --gradle-version $gradle_version
export GRADLEW_PATH="$(realpath gradlew)"

popd
