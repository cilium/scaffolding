package toolkit

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
)

// ListCiliumCliVersions returns list of cilium-cli releases using google's github library (github.com/google/go-github/github).
// Acts as a wrapper around `github.Client.Repositories.ListReleases` for convenience.
func ListCiliumCliVersions(ctx context.Context) ([]*github.RepositoryRelease, error) {
	client := github.NewClient(nil)

	releases, _, err := client.Repositories.ListReleases(
		ctx, "cilium", "cilium-cli", nil,
	)
	if err != nil {
		return nil, err
	}

	return releases, nil
}

// DownloadCiliumCliBin downloads a cilium-cli tarball release artifact from GitHub.
// Sha256 verification is performed on the downloaded tarball against the sha provided in the release artifacts.
func DownloadCiliumCliBin(
	logger *log.Logger,
	release *github.RepositoryRelease,
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

	logger.WithFields(
		log.Fields{
			"version":  release.GetTagName(),
			"platform": platform,
			"arch":     arch,
			"dest":     dest,
		},
	).Info("ensuring cilium-cli with following params")

	targetName := fmt.Sprintf("cilium-%s-%s.tar.gz", platform, arch)
	targetUrl := ""
	targetDest := filepath.Join(dest, targetName)
	targetShaName := targetName + ".sha256sum"
	targetShaUrl := ""
	targetShaDest := filepath.Join(dest, targetShaName)

	logger.WithFields(
		log.Fields{
			"shafile": targetShaDest,
			"bin":     targetDest,
		},
	).Debug("determined destinations on disk")

	for _, asset := range release.Assets {
		name := asset.GetName()
		if name == targetName {
			targetUrl = asset.GetBrowserDownloadURL()
		} else if name == targetShaName {
			targetShaUrl = asset.GetBrowserDownloadURL()
		}
		if targetUrl != "" && targetShaUrl != "" {
			break
		}
	}

	if targetUrl == "" || targetShaUrl == "" {
		return fmt.Errorf("unable to find asset urls for cilium-cli, results: bin: %s, sha: %s", targetUrl, targetShaUrl)
	}

	logger.WithFields(
		log.Fields{
			"cilium-cli-bin": targetUrl,
			"cilum-cli-sha":  targetShaUrl,
		},
	).Debug("downloading urls for cilium-cli bin")

	_, err := DownloadFile(targetShaDest, targetShaUrl, "", false)
	if err != nil {
		return fmt.Errorf("unable to download cilum-cli sha: %s", err)
	}

	binShaBytes, err := ioutil.ReadFile(targetShaDest)
	if err != nil {
		return fmt.Errorf("unable to read sha256 file: %s", err)
	}
	binSha := string(binShaBytes)
	// If we have <sha256> <filename>, just get the sha
	binSha = strings.Split(binSha, " ")[0]

	logger.WithField("sha256", binSha).Debug("expected sha256 for cilium-cli bin")

	_, err = DownloadFile(targetDest, targetUrl, string(binSha), true)
	if err != nil {
		return fmt.Errorf("unable to download cilium-cli bin: %s", err)
	}

	logger.Info("success")

	return err
}
