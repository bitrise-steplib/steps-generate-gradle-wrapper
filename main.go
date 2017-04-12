package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"path/filepath"

	"strings"

	"bufio"

	"github.com/bitrise-core/bitrise-init/scanners/android"
	"github.com/bitrise-core/bitrise-init/utility"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-android/sdk"
)

const (
	gradleDistributionURLFormat                  = "https://services.gradle.org/distributions/gradle-%s-bin.zip"
	gradleWrapperPropertiesDistributionURLFormat = `distributionUrl=https\://services.gradle.org/distributions/gradle-%s-all.zip`
)

var licenseMap = map[string]string{
	"android-sdk-license":         "\n8933bad161af4178b1185d1a37fbf41ea5269c55",
	"android-sdk-preview-license": "\n84831b9409646a918e30573bab4c9c91346d8abd",
	"intel-android-extra-license": "\nd975f751698a77b662f1254ddbeed3901e976f5a",
}

// ConfigsModel ...
type ConfigsModel struct {
	ProjectRootDir string
	GradleVersion  string
	AndroidHome    string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		ProjectRootDir: os.Getenv("project_root_dir"),
		GradleVersion:  os.Getenv("gradle_version"),
		AndroidHome:    os.Getenv("android_home"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Configs:")
	log.Printf("- ProjectRootDir: %s", configs.ProjectRootDir)
	log.Printf("- GradleVersion: %s", configs.GradleVersion)
	log.Printf("- AndroidHome: %s", configs.AndroidHome)
}

func (configs ConfigsModel) validate() error {
	if configs.ProjectRootDir == "" {
		return errors.New("no ProjectRootDir parameter specified")
	} else if exist, err := pathutil.IsPathExists(configs.ProjectRootDir); err != nil {
		return fmt.Errorf("failed to check if ProjectRootDir (%s) exists, error: %s", configs.ProjectRootDir, err)
	} else if !exist {
		return fmt.Errorf("ProjectRootDir (%s) not exists", configs.ProjectRootDir)
	}

	if configs.GradleVersion == "" {
		return errors.New("no GradleVersion parameter specified")
	}

	if configs.AndroidHome == "" {
		return errors.New("no AndroidHome parameter specified")
	}

	return nil
}

func downloadFileIntoTmpDir(url string) (string, error) {
	name := filepath.Base(url)

	// Download file to tmp dir
	tmpDir, err := pathutil.NormalizedOSTempDirPath("_generate_gradle_wrapper_")
	if err != nil {
		return "", fmt.Errorf("failed to create tmp destination dir, error %s", err)
	}
	tmpPth := filepath.Join(tmpDir, name)

	tmpFile, err := os.Create(tmpPth)
	defer func() {
		if err := tmpFile.Close(); err != nil {
			log.Warnf("Failed to close (%s)", tmpPth)
		}
	}()
	if err != nil {
		return "", fmt.Errorf("failed to create (%s), error: %s", tmpPth, err)
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download from (%s), error: %s", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warnf("failed to close (%s) body", url)
		}
	}()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to download from (%s), error: %s", url, err)
	}

	return tmpPth, nil
}

func unzipZipedArtifactDir(source, destination string) (string, error) {
	parentDir := filepath.Dir(source)
	fmt.Printf("parentDir: %s\n", parentDir)
	dirNameWithExt := filepath.Base(source)
	fmt.Printf("dirNameWithExt: %s\n", dirNameWithExt)
	dirName := strings.TrimSuffix(dirNameWithExt, "-bin"+filepath.Ext(dirNameWithExt))
	fmt.Printf("dirName: %s\n", dirName)
	deployPth := filepath.Join(destination, dirName)
	fmt.Printf("deployPth: %s\n", deployPth)

	cmd := command.New("/usr/bin/unzip", dirNameWithExt)
	cmd.SetDir(parentDir)

	fmt.Printf("$ %s\n", cmd.PrintableCommandArgs())

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to zip dir: %s, out: %s, error: %s", source, out, err)
	}

	return deployPth, nil
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	cmd := command.New("envman", "add", "--key", keyStr)
	cmd.SetStdin(strings.NewReader(valueStr))
	return cmd.Run()
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func main() {
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		fmt.Println()
		failf("Issue with input: %s", err)
	}

	//
	// Search for root build.gradle file
	fmt.Println()
	log.Infof("Search for root build.gradle file")

	fileList, err := utility.ListPathInDirSortedByComponents(configs.ProjectRootDir, false)
	if err != nil {
		failf("Failed to search for files in (%s), error: %s", configs.ProjectRootDir, err)
	}

	rootBuildGradleFiles, err := android.FilterRootBuildGradleFiles(fileList)
	if err != nil {
		failf("Failed to search for root build.gradle file, error: %s", err)
	}

	rootBuildGradlePth := ""

	if len(rootBuildGradleFiles) == 0 {
		failf("No root build.gradle file found")
	} else if len(rootBuildGradleFiles) > 1 {
		rootBuildGradlePth = rootBuildGradleFiles[0]

		log.Warnf("Multiple root build.gradle file found:")
		for _, pth := range rootBuildGradleFiles {
			log.Warnf("- %s", pth)
		}
	} else {
		rootBuildGradlePth = rootBuildGradleFiles[0]
	}

	log.Donef("root build.gradle path: %s", rootBuildGradlePth)
	// ---

	rootBuildGradleParentDir := filepath.Dir(rootBuildGradlePth)
	gradlewPth := filepath.Join(rootBuildGradleParentDir, "gradlew")
	if exist, err := pathutil.IsPathExists(gradlewPth); err != nil {
		failf("Failed to check if gradlew exist at: %s, error: %s", gradlewPth, err)
	} else if exist {
		log.Donef("Gradle Wrapper exist at: %s", gradlewPth)
		return
	}

	//
	// Ensure licenses
	fmt.Println()
	log.Infof("Ensure Android SDK Licenses")

	sdk, err := sdk.New(configs.AndroidHome)
	if err != nil {
		failf("Failed to create sdk, error: %s", err)
	}

	licencesDir := filepath.Join(sdk.GetAndroidHome(), "licenses")
	if exist, err := pathutil.IsDirExists(licencesDir); err != nil {
		failf("Failed to check if licences dir exist at: %s, error: %s", licencesDir, err)
	} else if !exist {
		log.Printf("licenses dir not exist, generating...")
		if err := os.Mkdir(licencesDir, 0777); err != nil {
			failf("Failed to create dir at: %s, error: %s", licencesDir, err)
		}
	}

	for licenceKey, licenseValue := range licenseMap {
		licencePth := filepath.Join(licencesDir, licenceKey)
		if exist, err := pathutil.IsPathExists(licencePth); err != nil {
			failf("Failed to check if license exist at: %s, error: %s", licencePth, err)
		} else if !exist {
			log.Printf("%s not exist, generating...", licenceKey)
			if err := fileutil.WriteStringToFile(licencePth, licenseValue); err != nil {
				failf("Failed to write license, error: %s", err)
			}
			log.Donef("%s generated", licenceKey)
		} else {
			log.Donef("%s exist", licenceKey)
		}
	}
	// ---

	//
	// Generate Gradle Wrapper
	fmt.Println()
	log.Infof("Generate Gradle Wrapper")

	wrapperDir := filepath.Join(sdk.GetAndroidHome(), "tools", "templates", "gradle", "wrapper")
	if exist, err := pathutil.IsDirExists(wrapperDir); err != nil {
		failf("Failed to check if path: %s exists, error: %s", wrapperDir, err)
	} else if !exist {
		failf("gradle wrapper template not exists at: %s", wrapperDir)
	}

	gradlewInSDKPth := filepath.Join(wrapperDir, "gradlew")
	if err := command.CopyFile(gradlewInSDKPth, gradlewPth); err != nil {
		failf("Failed to copy gradlew from: %s to: %s", gradlewInSDKPth, gradlewPth)
	}

	gradleDir := filepath.Join(rootBuildGradleParentDir, "gradle")
	if err := os.Mkdir(gradleDir, 0777); err != nil {
		failf("Failed to create: %s, error: %s", gradleDir, err)
	}

	gradleWrapperDirInSDK := filepath.Join(wrapperDir, "gradle", "wrapper")
	if err := command.CopyDir(gradleWrapperDirInSDK, gradleDir, false); err != nil {
		failf("Failed to copy: %s to %s, error: %s", gradleWrapperDirInSDK, gradleDir, err)
	}

	gradleWrapperPropertiesPth := filepath.Join(gradleDir, "wrapper", "gradle-wrapper.properties")
	gradleWrapperPropertiesContent, err := fileutil.ReadStringFromFile(gradleWrapperPropertiesPth)
	if err != nil {
		failf("Failed to read %s, error: %s", gradleWrapperPropertiesPth, err)
	}

	updatedLines := []string{}
	reader := strings.NewReader(gradleWrapperPropertiesContent)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "distributionUrl") {
			line = fmt.Sprintf(gradleWrapperPropertiesDistributionURLFormat, configs.GradleVersion)
		}
		updatedLines = append(updatedLines, line)
	}

	if err := fileutil.WriteStringToFile(gradleWrapperPropertiesPth, strings.Join(updatedLines, "\n")); err != nil {
		failf("Failed to update gradle-wrapper.properties, error: %s", err)
	}

	if err := exportEnvironmentWithEnvman("GRADLEW_PATH", gradlewPth); err != nil {
		failf("Failed to export gradlew path into GRADLEW_PATH environment")
	}

	log.Donef("Gradle Wrapper generated: %s", gradlewPth)
	// ---

}
