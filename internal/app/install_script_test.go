package app

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptDownloadsLatestDebAndRunsDpkg(t *testing.T) {
	temp := t.TempDir()
	binDir := filepath.Join(temp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(temp, "calls.log")
	writeExecutable := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable("id", "echo 0\n")
	writeExecutable("curl", `
log="$F_INSTALL_TEST_LOG"
printf 'curl %s\n' "$*" >>"$log"
case "$*" in
  *api.github.com*) printf '%s\n' '{"tag_name":"v0.9.5"}'; exit 0 ;;
esac
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; break; fi
  prev="$arg"
done
[ -n "$out" ] || exit 1
printf 'fake-deb' >"$out"
`)
	writeExecutable("dpkg", "printf 'dpkg %s\\n' \"$*\" >>\"$F_INSTALL_TEST_LOG\"\n")

	cmd := exec.Command("sh", "../../install.sh")
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"HOME="+filepath.Join(temp, "home"),
		"F_INSTALL_TEST_LOG="+logPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := string(raw)
	for _, want := range []string{
		"api.github.com/repos/sergeybychkovvvpgroup-beep/f/releases/latest",
		"f_0.9.5_linux_amd64.deb",
		"dpkg -i",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("installer did not perform %q:\n%s\noutput:\n%s", want, calls, output)
		}
	}
}

func TestInstallScriptFallsBackToUserArchiveWhenSudoDpkgFails(t *testing.T) {
	temp := t.TempDir()
	binDir := filepath.Join(temp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(temp, "release.tar.gz")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)
	binary := []byte("#!/bin/sh\necho 'f 0.9.5'\n")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "f", Mode: 0o755, Size: int64(len(binary))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	writeExecutable := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable("id", "echo 1000\n")
	writeExecutable("sudo", "exit 1\n")
	writeExecutable("dpkg", "exit 99\n")
	writeExecutable("curl", `
case "$*" in
  *api.github.com*) printf '%s\n' '{"tag_name":"v0.9.5"}'; exit 0 ;;
esac
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; break; fi
  prev="$arg"
done
case "$*" in
  *.tar.gz*) cp "$F_INSTALL_TEST_ARCHIVE" "$out" ;;
  *) printf 'fake-deb' >"$out" ;;
esac
`)

	installDir := filepath.Join(temp, "install")
	cmd := exec.Command("sh", "../../install.sh")
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"HOME="+filepath.Join(temp, "home"),
		"F_BIN_DIR="+installDir,
		"F_INSTALL_TEST_ARCHIVE="+archivePath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh did not fall back after sudo dpkg failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(installDir, "f")); err != nil {
		t.Fatalf("user-local binary was not installed: %v\n%s", err, output)
	}
}
