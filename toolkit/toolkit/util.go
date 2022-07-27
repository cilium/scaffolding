package toolkit

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

//https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07
func ExtractFromTar(tarball string, file string, dest string) error {
	dest, _ = filepath.Abs(dest)

	if !PathIsDir(dest) {
		return fmt.Errorf(
			"expected dest to be path to existing directory, got something else: %s",
			dest,
		)
	}

	if PathIsDir(tarball) || !PathExists(tarball) {
		return fmt.Errorf(
			"expected tar to be path to existing tarball, got something else: %s",
			tarball,
		)
	}

	destPath := filepath.Join(dest, file)

	tfr, err := os.Open(tarball)
	if err != nil {
		return err
	}

	gzr, err := gzip.NewReader(tfr)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		// if no more files in tarball, and we
		// haven't found our file, error out
		case err == io.EOF:
			return fmt.Errorf(
				"unable to find file %s in tarball %s",
				file, tarball,
			)
		// return any other error
		case err != nil:
			return err
		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		case header.Typeflag == tar.TypeDir:
			continue
		case filepath.Base(header.Name) != file:
			continue
		}

		// found our file
		f, err := os.OpenFile(
			destPath,
			os.O_CREATE|os.O_RDWR,
			os.FileMode(header.Mode),
		)
		if err != nil {
			return err
		}

		// copy over contents
		if _, err := io.Copy(f, tr); err != nil {
			return err
		}

		// manually close here after each file operation; defering would cause each file close
		// to wait until all operations have completed.
		f.Close()
		return nil
	}
}

func ExitWithError(logger *log.Logger, err error) {
	logger.WithFields(
		log.Fields{
			"err": err,
		},
	).Error("❗something really bad happened, unable to continue")
	os.Exit(1)
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func PathIsDir(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}

func PPrint(n interface{}) error {
	marshalled, err := json.MarshalIndent(n, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(marshalled))
	return nil
}

func RandomString(n int) string {
	sb := strings.Builder{}
	for n > 0 {
		index := rand.Intn(26)
		sb.WriteString(fmt.Sprintf("%c", 97+index))
		n--
	}
	return sb.String()
}

func SimpleRetry(target func() error, maxAttempts int, delay time.Duration) error {
	attempts := 0
	var err error = nil

	for attempts < maxAttempts {
		err = target()
		if err == nil {
			return nil
		}
		time.Sleep(delay)
		attempts++
	}

	return err
}

func GetFileSHA256(target string) (string, error) {
	f, err := os.Open(target)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func DownloadFile(dest string, url string, expectedSHA256 string, idempotent ...bool) (int, error) {
	check_exists := true

	if len(idempotent) > 0 {
		if !idempotent[0] {
			check_exists = false
		}
	}

	if check_exists {
		if expectedSHA256 == "" {
			return 0, errors.New("unable to idempotently download file, no sha256 given")
		}
		_, err := os.Stat(dest)
		if err == nil {
			destSHA256, err := GetFileSHA256(dest)
			if err != nil {
				return 0, err
			}
			if destSHA256 == expectedSHA256 {
				return 0, nil
			}
		}
	}

	outFile, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	var resp *http.Response
	err = SimpleRetry(
		func() error {
			resp, err = http.Get(url)
			return err
		},
		3, 5*time.Second,
	)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bad status from %s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if expectedSHA256 != "" {
		h := sha256.New()
		if _, err := h.Write(body); err != nil {
			return 0, err
		}
		resultSHA256 := hex.EncodeToString(h.Sum(nil))
		if resultSHA256 != expectedSHA256 {
			return 0, fmt.Errorf(
				"sha256 mismatch from %s: got: %s, expected: %s",
				url,
				resultSHA256,
				expectedSHA256,
			)
		}
	}

	n, err := outFile.Write(body)
	if err != nil {
		return 0, err
	}

	return n, nil
}

func ExecInPod() {}
