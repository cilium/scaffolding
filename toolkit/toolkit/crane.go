package toolkit

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
)

func GetLatestCraneRelease(ctx context.Context) (*github.RepositoryRelease, error) {
	client := github.NewClient(nil)

	release, _, err := client.Repositories.GetLatestRelease(
		ctx, "google", "go-containerregistry",
	)
	if err != nil {
		return nil, err
	}

	return release, nil
}

func DownloadCraneBin(
	ctx context.Context,
	logger *log.Logger,
	platform string,
	arch string,
	dest string,
) error {
	dest, _ = filepath.Abs(dest)
	if !PathExists(dest) {
		return fmt.Errorf(
			"given destination does not exist: %s",
			dest,
		)
	} else if !PathIsDir(dest) {
		return fmt.Errorf(
			"given destination is not a dir: %s",
			dest,
		)
	}

	latestRelease, err := GetLatestCraneRelease(ctx)
	if err != nil {
		return fmt.Errorf("unable to get latest crane release from google/go-containerregistry: %s", err)
	}

	// for this project, they use x86_64 over amd64
	// correct for convenience
	if arch == "amd64" {
		arch = "x86_64"
	}
	// they also capitalize the platform
	platform = strings.Title(strings.ToLower(platform))

	logger.WithFields(
		log.Fields{
			"version":  latestRelease.GetTagName(),
			"platform": platform,
			"arch":     arch,
			"dest":     dest,
		},
	).Info("ensuring crane bin with following params")

	targetName := fmt.Sprintf(
		"go-containerregistry_%s_%s.tar.gz",
		platform, arch,
	)
	targetUrl := ""
	targetDest := filepath.Join(dest, targetName)
	checksumName := "checksums.txt"
	checksumUrl := ""
	checksumDest := filepath.Join(dest, checksumName)

	logger.WithFields(
		log.Fields{
			"checksum-file": checksumDest,
			"bin":           targetDest,
		},
	).Debug("determined destinations on disk")

	for _, asset := range latestRelease.Assets {
		name := asset.GetName()
		if name == targetName {
			targetUrl = asset.GetBrowserDownloadURL()
		} else if name == checksumName {
			checksumUrl = asset.GetBrowserDownloadURL()
		}
		if targetUrl != "" && checksumUrl != "" {
			break
		}
	}

	if targetUrl == "" || checksumUrl == "" {
		return fmt.Errorf("unable to find asset urls for crane, results: bin: %s, checksums: %s", targetUrl, checksumUrl)
	}

	logger.WithFields(
		log.Fields{
			"checksum-file": checksumUrl,
			"bin":           targetUrl,
		},
	).Debug("downloading urls for crane bin")

	_, err = DownloadFile(checksumDest, checksumUrl, "", false)
	if err != nil {
		return fmt.Errorf("unable to download checksum file: %s", err)
	}

	// now we need to find the right checksum within the file
	// has the same format as sha256sum: sha256<whitespace>filename
	// thank you to: https://stackoverflow.com/questions/8757389/reading-a-file-line-by-line-in-go
	logger.Debug("looking for sha256 in checksums.txt")
	checksumHandler, err := os.Open(checksumDest)
	if err != nil {
		return fmt.Errorf("unable to open checksums.txt: %s", err)
	}
	defer checksumHandler.Close()
	scanner := bufio.NewScanner(checksumHandler)

	targetSha := ""
	for scanner.Scan() {
		line := strings.Split(scanner.Text(), "  ")
		sha, file := line[0], line[1]
		if file == targetName {
			targetSha = sha
			break
		}
	}
	if err = scanner.Err(); err != nil {
		return fmt.Errorf("something bad happened while reading checksums.txt: %s", err)
	}
	if targetSha == "" {
		return fmt.Errorf("unable to find %s in checksums file", targetName)
	}

	logger.WithField("sha256", targetSha).Debug("expected sha256 for crane tarball")

	_, err = DownloadFile(targetDest, targetUrl, targetSha, true)
	if err != nil {
		return fmt.Errorf("unable to download crane tarball: %s", err)
	}

	logger.Info("extracting crane bin from tarball")
	err = ExtractFromTar(
		targetName,
		"crane",
		dest,
	)
	if err != nil {
		return fmt.Errorf("unable to extract crane bin from tarball: %s", err)
	}
	logger.Info("success")

	return err
}
