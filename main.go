package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-steputils/tools"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

const (
	gradleWrapperPathEnvKey = "GRADLEW_PATH"
)

// Configs ...
type Configs struct {
	ProjectRootDir string `env:"project_root_dir,dir"`
	GradleVersion  string `env:"gradle_version,required"`
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func main() {
	var config Configs

	if err := stepconf.Parse(&config); err != nil {
		failf("Couldn't create step config: %v", err)
	}

	stepconf.Print(config)
	fmt.Println()

	gradleCommand := command.New("gradle", "wrapper", "--gradle-version", config.GradleVersion)
	gradleCommand.SetDir(config.ProjectRootDir)
	if out, err := gradleCommand.RunAndReturnTrimmedCombinedOutput(); err != nil {
		failf("%s failed with error %v: %s", gradleCommand.PrintableCommandArgs(), err, out)
	}

	gradlewPath := filepath.Join(config.ProjectRootDir, "gradlew")
	exists, err := pathutil.IsPathExists(gradlewPath)
	if !exists || err != nil {
		failf("Gradle command passed but cannot find generated gradlew file, error: %v", err)
	}

	if err := tools.ExportEnvironmentWithEnvman(gradleWrapperPathEnvKey, gradlewPath); err != nil {
		failf("Failed to export environment variable: %s", gradleWrapperPathEnvKey)
	}

	log.Donef("Gradle Wrapper generated: %s", gradlewPath)
}
