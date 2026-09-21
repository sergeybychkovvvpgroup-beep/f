package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const updateCheckInterval = 15 * time.Minute

type updateCheckCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Commit    string    `json:"commit"`
}

func maybeOfferUpgrade(stdin io.Reader, stdout, stderr io.Writer) error {
	latest, err := latestUpgradeCommit(defaultUpgradeRepo())
	if err != nil || latest == "" || commitsEqual(buildCommit, latest) {
		return err
	}

	fmt.Fprintf(stdout, "[f] Доступно обновление (%s -> %s). Обновить сейчас? [Y/n] ", shortCommit(buildCommit), shortCommit(latest))
	answer, readErr := bufio.NewReader(stdin).ReadString('\n')
	if readErr != nil && readErr != io.EOF {
		return readErr
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer == "n" || answer == "no" || answer == "нет" {
		return nil
	}
	if answer != "" && answer != "y" && answer != "yes" && answer != "д" && answer != "да" {
		fmt.Fprintln(stdout, "[f] Обновление пропущено")
		return nil
	}

	if err := runUpgrade(nil, stdout, stderr); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "[f] Обновление установлено; новая версия запустится в следующий раз")
	return nil
}

func latestUpgradeCommit(repoURL string) (string, error) {
	cachePath, err := updateCachePath()
	if err == nil {
		if cached, ok := readUpdateCache(cachePath); ok && time.Since(cached.CheckedAt) < updateCheckInterval {
			return cached.Commit, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", repoURL, "HEAD")
	output, cmdErr := cmd.Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("repository check timed out")
	}
	if cmdErr != nil {
		return "", fmt.Errorf("check repository: %w", cmdErr)
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return "", fmt.Errorf("repository returned no HEAD commit")
	}
	commit := strings.TrimSpace(fields[0])
	if cachePath != "" {
		_ = writeUpdateCache(cachePath, updateCheckCache{CheckedAt: time.Now(), Commit: commit})
	}
	return commit, nil
}

func updateCachePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "aoo", "update-check.json"), nil
}

func readUpdateCache(path string) (updateCheckCache, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return updateCheckCache{}, false
	}
	var cached updateCheckCache
	if json.Unmarshal(raw, &cached) != nil || cached.CheckedAt.IsZero() || strings.TrimSpace(cached.Commit) == "" {
		return updateCheckCache{}, false
	}
	return cached, true
}

func writeUpdateCache(path string, cached updateCheckCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".update-check-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func commitsEqual(current, latest string) bool {
	current = strings.TrimSpace(current)
	latest = strings.TrimSpace(latest)
	if current == "" || latest == "" || current == "unknown" {
		return false
	}
	return current == latest || strings.HasPrefix(current, latest) || strings.HasPrefix(latest, current)
}

func shortCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return "unknown"
	}
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}
