package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"modernc.org/sqlite"
)

// Callers confine parent directories to their configured root; SQLite also
// must not follow a symlink at the database or one of its sidecars.
func validatePath(path string) error {
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink database path is not allowed: %s", path)
	}
	return nil
}

func open(path, mode string) (*sql.DB, error) {
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if err := validatePath(path + suffix); err != nil {
			return nil, err
		}
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("mode", mode)
	u.RawQuery = q.Encode()
	d, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)
	return d, nil
}

// offline retains SQLite's exclusive lock until Close. Switching out of WAL
// refuses even idle WAL connections; busy writers/readers are never replaced.
func offline(path string) (*sql.DB, error) {
	d, err := open(path, "rwc")
	if err != nil {
		return nil, err
	}
	var mode string
	if err = d.QueryRow("PRAGMA journal_mode=DELETE").Scan(&mode); err == nil && mode != "delete" {
		err = fmt.Errorf("database is not offline")
	}
	if err == nil {
		_, err = d.Exec("PRAGMA locking_mode=EXCLUSIVE; BEGIN EXCLUSIVE; COMMIT")
	}
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("destination must be offline: %w", err)
	}
	return d, nil
}

// CopyDB takes a native page backup of a live source, preserving implicit
// ROWIDs, then restores through SQLite while retaining the destination inode.
func CopyDB(src, dst string) error {
	source, err := open(src, "ro")
	if err != nil {
		return err
	}
	defer source.Close()
	if err = validatePath(dst); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".spin-snapshot-*.db")
	if err != nil {
		return err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)
	sourceConn, err := source.Conn(context.Background())
	if err != nil {
		return err
	}
	err = sourceConn.Raw(func(raw any) error {
		backup, err := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		}).NewBackup(name)
		if err != nil {
			return err
		}
		_, err = backup.Step(-1)
		finish := backup.Finish()
		if err != nil {
			return err
		}
		return finish
	})
	sourceConn.Close()
	if err != nil {
		return fmt.Errorf("snapshot source: %w", err)
	}
	target, err := offline(dst)
	if err != nil {
		return err
	}
	defer target.Close()
	conn, err := target.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(raw any) error {
		backup, err := raw.(interface {
			NewRestore(string) (*sqlite.Backup, error)
		}).NewRestore(name)
		if err != nil {
			return err
		}
		_, err = backup.Step(-1)
		finish := backup.Finish()
		if err != nil {
			return err
		}
		return finish
	})
}

// RemoveDB requires an offline database before unlinking it and its sidecars.
func RemoveDB(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	d, err := offline(path)
	if err != nil {
		return err
	}
	defer d.Close()
	for _, suffix := range []string{"-shm", "-wal", "-journal", ""} {
		if err := os.Remove(path + suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
