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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
	keep bool,
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
	caser := cases.Title(language.AmericanEnglish)
	platform = caser.String(strings.ToLower(platform))

	logger.WithFields(
		log.Fields{
			"version":  latestRelease.GetTagName(),
			"platform": platform,
			"arch":     arch,
			"dest":     dest,
		},
	).Info("ensuring crane bin with following params")

	tarballName := fmt.Sprintf(
		"go-containerregistry_%s_%s.tar.gz",
		platform, arch,
	)
	tarballUrl := ""
	tarballDest := filepath.Join(dest, tarballName)
	checksumName := "checksums.txt"
	checksumUrl := ""
	checksumDest := filepath.Join(dest, checksumName)

	logger.WithFields(
		log.Fields{
			"checksum-file": checksumDest,
			"tarball":       tarballDest,
		},
	).Debug("determined destinations on disk")

	for _, asset := range latestRelease.Assets {
		name := asset.GetName()
		if name == tarballName {
			tarballUrl = asset.GetBrowserDownloadURL()
		} else if name == checksumName {
			checksumUrl = asset.GetBrowserDownloadURL()
		}
		if tarballUrl != "" && checksumUrl != "" {
			break
		}
	}

	if tarballUrl == "" || checksumUrl == "" {
		return fmt.Errorf("unable to find asset urls for crane, results: bin: %s, checksums: %s", tarballUrl, checksumUrl)
	}

	logger.WithFields(
		log.Fields{
			"checksum-file": checksumUrl,
			"tarball":       tarballUrl,
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

	tarballSha := ""
	for scanner.Scan() {
		line := strings.Split(scanner.Text(), "  ")
		sha, file := line[0], line[1]
		if file == tarballName {
			tarballSha = sha
			break
		}
	}
	if err = scanner.Err(); err != nil {
		return fmt.Errorf("something bad happened while reading checksums.txt: %s", err)
	}
	if tarballSha == "" {
		return fmt.Errorf("unable to find %s in checksums file", tarballName)
	}

	logger.WithField("sha256", tarballSha).Debug("expected sha256 for crane tarball")
	bytesWritten, err := DownloadFile(tarballDest, tarballUrl, tarballSha, true)
	if err != nil {
		return fmt.Errorf("unable to download crane tarball: %s", err)
	}

	if bytesWritten == 0 {
		logger.WithFields(
			log.Fields{
				"sha256":  tarballSha,
				"tarball": tarballDest,
			},
		).Info("did not download crane tarball, already present")
	}

	logger.Info("extracting crane bin from tarball")
	err = ExtractFromTar(
		tarballDest,
		"crane",
		dest,
	)
	if err != nil {
		return fmt.Errorf("unable to extract crane bin from tarball: %s", err)
	}

	if !keep {
		logger.Info("cleaning up")
		logger.WithField("tarball", tarballDest).Debug("removing tarball")
		err = os.Remove(tarballDest)
		if err != nil {
			return fmt.Errorf("unable to remove crane tarball: %s", err)
		}
		logger.WithField("checksums", checksumDest).Debug("removing checksums file")
		err = os.Remove(checksumDest)
		if err != nil {
			return fmt.Errorf("unable to remove crane checksums file: %s", err)
		}
	}

	logger.Info("success")

	return err
}
