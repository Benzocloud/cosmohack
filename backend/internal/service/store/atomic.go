package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func replaceFile(path string, payload []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return fmt.Errorf("tmp name: %w", err)
	}
	tmp := filepath.Join(dir, ".tmp."+hex.EncodeToString(rnd[:]))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(payload); err != nil {
		_ = f.Close()
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		// Windows часто не даёт Rename поверх существующего файла.
		bak := path + ".old"
		_ = os.Remove(bak)
		if err2 := os.Rename(path, bak); err2 != nil && !os.IsNotExist(err2) {
			return fmt.Errorf("park dest: %w", err2)
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Rename(bak, path)
			return fmt.Errorf("rename: %w", err)
		}
		_ = os.Remove(bak)
	}
	ok = true
	return nil
}

func removeTree(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func cleanTmp(root string) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if len(base) >= 5 && base[:5] == ".tmp." {
			_ = os.Remove(path)
		}
		return nil
	})
}
