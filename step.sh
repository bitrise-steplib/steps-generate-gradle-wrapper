#!/bin/bash

set -e

cd $project_root_dir

gradle wrapper --gradle-version $gradle_version
export GRADLEW_PATH="$(realpath gradlew)"

echo "Gradle wrapper is available at $GRADLEW_PATH"
