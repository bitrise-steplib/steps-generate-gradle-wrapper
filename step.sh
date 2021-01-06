#!/bin/bash

set -e

cd $project_root_dir

gradle wrapper --gradle-version $gradle_version

GRADLEW_PATH="$(realpath gradlew)"
envman add --key GRADLEW_PATH --value "$GRADLEW_PATH"

echo "Gradle wrapper is available at $GRADLEW_PATH"
