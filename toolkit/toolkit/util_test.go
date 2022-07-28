package toolkit

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func errHasPrefix(err error, prefix string) bool {
	return strings.HasPrefix(err.Error(), prefix)
}

func TestExtractFromTarRaisesErrorOnFileNotFound(t *testing.T) {
	tmpdir := t.TempDir()

	failWithError := func(err error) {
		t.Error(err)
		t.FailNow()
	}

	newTestTarball := func() (string, string, []byte) {
		testFileName := "testfile"
		testFileContent := []byte("i am a test file")
		testFilePath := filepath.Join(tmpdir, testFileName)
		os.WriteFile(
			testFilePath,
			testFileContent,
			0755,
		)
		testFileStat, err := os.Lstat(testFilePath)
		if err != nil {
			failWithError(err)
		}

		tarPath := filepath.Join(tmpdir, "testfile.tar.gz")
		tarWriter, err := os.Create(tarPath)
		if err != nil {
			failWithError(err)
		}

		gzw := gzip.NewWriter(tarWriter)
		defer gzw.Close()
		tw := tar.NewWriter(gzw)
		defer tw.Close()

		header, err := tar.FileInfoHeader(testFileStat, testFileStat.Name())
		if err != nil {
			failWithError(err)
		}

		if err := tw.WriteHeader(header); err != nil {
			failWithError(err)
		}

		testFileReader, err := os.Open(testFilePath)
		if err != nil {
			failWithError(err)
		}
		defer testFileReader.Close()

		if _, err := io.Copy(tw, testFileReader); err != nil {
			failWithError(err)
		}

		if err = os.Remove(testFilePath); err != nil {
			failWithError(err)
		}

		return tarPath, testFileName, testFileContent
	}

	tb, tf, tc := newTestTarball()
	err := ExtractFromTar(tb, "idontexist", tmpdir)
	if err == nil {
		t.Error("no error raised when target file not found in tarball")
		t.FailNow()
	}
	if !errHasPrefix(err, "unable to find file") {
		t.Error("unexpected error raised when target file not in tarball")
		t.Error(err)
		t.FailNow()
	}

	err = ExtractFromTar(tb, tf, tmpdir)
	if err != nil {
		failWithError(err)
	}

	extractedContent, err := os.ReadFile(
		filepath.Join(tmpdir, tf),
	)
	if err != nil {
		failWithError(err)
	}
	if string(extractedContent) != string(tc) {
		t.Errorf("extracted content does not match expected content: %s", extractedContent)
		t.FailNow()
	}
}

func TestExtractFromTarRaisesErrorsOnInputValidationFail(t *testing.T) {
	tmpdir := t.TempDir()
	tfPath := filepath.Join(tmpdir, "testfile")
	os.WriteFile(
		tfPath,
		[]byte("i am a test file"),
		0755,
	)

	var err error

	// raise an error if dest does not exist
	err = ExtractFromTar("", "", tmpdir+"idontexist")
	if !errHasPrefix(err, "given destination directory does not exist") {
		t.Error("unable to validate destination dir exists")
		t.FailNow()
	}

	// raise an error if dest is not a directory
	err = ExtractFromTar("", "", tfPath)
	if !errHasPrefix(err, "given destination directory is not actually a directory") {
		t.Error("unable to validate destination dir is a dir")
		t.FailNow()
	}

	// raise an error if tarball does not exist
	err = ExtractFromTar(tmpdir+"idontexist", "", tmpdir)
	if !errHasPrefix(err, "given tarball does not exist") {
		t.Error("unable to validate tarball exists")
	}

	// raise an error if tarball is dir
	err = ExtractFromTar(tmpdir, "", tmpdir)
	if !errHasPrefix(err, "given tarball is actually a directory") {
		t.Error("unable to validate tarball is not a directory")
	}
}

func TestPathExistsReturnsCorrectBoolForPath(t *testing.T) {
	tmpdir := t.TempDir()
	if !PathExists(tmpdir) {
		t.Fail()
	}
	if PathExists(tmpdir + "idontexist") {
		t.Fail()
	}
}

func TestRandomStringUsesWholeAlphabet(t *testing.T) {
	containsFalse := func(slc []bool) bool {
		for _, b := range slc {
			if !b {
				return false
			}
		}
		return true
	}

	alpha := make([]bool, 26)
	alpha[0] = false

	maxLoops := 1
	var randomStr string
	for !containsFalse(alpha) && maxLoops > 0 {
		randomStr = RandomString(26)
		for _, c := range randomStr {
			alpha[int(c)-97] = true
		}
		maxLoops--
	}

	if !containsFalse(alpha) {
		return
	}
	t.Fail()
}

func TestRandomStringRespectsGivenLength(t *testing.T) {
	i := 0
	var randomStr string
	for i < 100 {
		randomStr = RandomString(i)
		if len(randomStr) != i {
			t.Errorf("expected str of len %d", i)
			t.FailNow()
		}
		i++
	}
}

func TestSimpleRetryRespectsMaxAttempts(t *testing.T) {
	var counter *int = new(int)
	*counter = 0

	start := time.Now()
	SimpleRetry(
		func() error {
			*counter += 1
			return errors.New("my-fake-error")
		},
		5,
		100*time.Millisecond,
	)
	end := time.Now()

	if *counter != 5 {
		t.Errorf("did not retry 5 times: %d", *counter)
		t.FailNow()
	}

	duration := end.Sub(start)
	if duration < 5*100*time.Millisecond {
		t.Errorf("duration is less than 500 milliseconds: %s", duration)
		t.FailNow()
	}
	if duration > 6*100*time.Millisecond {
		t.Errorf("duration is greater than 600 milliseconds: %s", duration)
		t.FailNow()
	}
}

func TestGetFileSHA256GivesCorrectSHA(t *testing.T) {
	tmp := t.TempDir()
	testfile := filepath.Join(tmp, "mysha256test")
	os.WriteFile(
		testfile,
		[]byte(
			"Bedevere: \"What makes you think she is a witch?\"\n"+
				"Peasant: \"She turned me into a newt.\"\n"+
				"Bedevere: \"A newt?\"\n"+
				"Peasant: \"Well I got better.\"\n",
		),
		0644,
	)
	expectedSHA := "1d4e23f92a86fee91c7ace160f80e546794d70e5874bf3890bf37b6c3989d221"

	result, err := GetFileSHA256(testfile)
	if err != nil {
		t.Error(err)
	} else if result != expectedSHA {
		t.Errorf("sha256 mismatch: expected %s, got %s", expectedSHA, result)
	}
}

func TestDownloadFileCanIdempotentFetchRemoteURLAndCheckSum(t *testing.T) {
	tmp := t.TempDir()

	targetUrl := "https://raw.githubusercontent.com/cilium/cilium/7151b180b985aeaa438375939a2e2682831e88f4/README.rst"
	targetSHA256 := "56de413502f6a98ac0a8c4f277e7be4c66f597b71a68b4211df8bd5ed5a1bfbc"

	dst := filepath.Join(tmp, "README.rst")
	n, err := DownloadFile(dst, targetUrl, targetSHA256)

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	dstSha, err := GetFileSHA256(dst)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if dstSha != targetSHA256 {
		t.Errorf("sha256 mismatch: expected: %s, got: %s", dstSha, targetSHA256)
		t.FailNow()
	}

	if n == 0 {
		t.Error("expected bytes to be written, got 0")
		t.FailNow()
	}

	n, err = DownloadFile(dst, targetUrl, targetSHA256, true)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if n != 0 {
		t.Errorf("failed idempotent check")
	}
}
