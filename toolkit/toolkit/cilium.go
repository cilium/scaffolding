package toolkit

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
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

	logger.WithFields(
		log.Fields{
			"version":  release.GetTagName(),
			"platform": platform,
			"arch":     arch,
			"dest":     dest,
		},
	).Info("ensuring cilium-cli with following params")

	tarballName := fmt.Sprintf("cilium-%s-%s.tar.gz", platform, arch)
	tarballUrl := ""
	tarballDest := filepath.Join(dest, tarballName)
	tarballShaName := tarballName + ".sha256sum"
	tarballShaUrl := ""
	tarballShaDest := filepath.Join(dest, tarballShaName)

	logger.WithFields(
		log.Fields{
			"shafile": tarballShaDest,
			"bin":     tarballDest,
		},
	).Debug("determined destinations on disk")

	for _, asset := range release.Assets {
		name := asset.GetName()
		if name == tarballName {
			tarballUrl = asset.GetBrowserDownloadURL()
		} else if name == tarballShaName {
			tarballShaUrl = asset.GetBrowserDownloadURL()
		}
		if tarballUrl != "" && tarballShaUrl != "" {
			break
		}
	}

	if tarballUrl == "" || tarballShaUrl == "" {
		return fmt.Errorf("unable to find asset urls for cilium-cli, results: bin: %s, sha: %s", tarballUrl, tarballShaUrl)
	}

	logger.WithFields(
		log.Fields{
			"cilium-cli-bin": tarballUrl,
			"cilum-cli-sha":  tarballShaUrl,
		},
	).Debug("downloading urls for cilium-cli bin")

	_, err := DownloadFile(tarballShaDest, tarballShaUrl, "", false)
	if err != nil {
		return fmt.Errorf("unable to download cilum-cli sha: %s", err)
	}

	tarballShaBytes, err := ioutil.ReadFile(tarballShaDest)
	if err != nil {
		return fmt.Errorf("unable to read sha256 file: %s", err)
	}
	tarballSha := string(tarballShaBytes)
	// If we have <sha256> <filename>, just get the sha
	tarballSha = strings.Split(tarballSha, " ")[0]

	logger.WithField("sha256", tarballSha).Debug("expected sha256 for cilium-cli tarball")

	bytesWritten, err := DownloadFile(tarballDest, tarballUrl, string(tarballSha), true)
	if err != nil {
		return fmt.Errorf("unable to download cilium-cli tarball: %s", err)
	}

	if bytesWritten == 0 {
		logger.WithFields(
			log.Fields{
				"sha256":  tarballSha,
				"tarball": tarballDest,
			},
		).Info("did not download cilium tarball, already present")
	}

	logger.Info("extracting cilium bin from tarball")
	err = ExtractFromTar(
		tarballDest,
		"cilium",
		dest,
	)
	if err != nil {
		return fmt.Errorf("unable to extract cilium-cli bin from tarball: %s", err)
	}

	if !keep {
		logger.Info("cleaning up")

		logger.WithField("tarball", tarballDest).Debug("removing tarball")
		err = os.Remove(tarballDest)
		if err != nil {
			return fmt.Errorf("unable to remove cilium tarball: %s", err)
		}

		logger.WithField("checksum", tarballShaDest).Debug("removing checksum")
		err = os.Remove(tarballShaDest)
		if err != nil {
			return fmt.Errorf("unable to remove cilium tarball checksum file: %s", err)
		}
	}
	logger.Info("success")

	return err
}
