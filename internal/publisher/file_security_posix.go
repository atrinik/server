// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

//go:build !windows

package publisher

import (
	"errors"
	"os"
	"path"
)

func openOwnerOnlyFile(root *os.Root, name string) (*os.File, error) {
	if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
		return nil, err
	}
	file, err := root.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := validateOwnerOnlyFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func validateOwnerOnlyFile(file *os.File) error {
	information, err := file.Stat()
	if err != nil || information.Mode().Perm()&0o077 != 0 {
		return errors.New("file is not owner-only")
	}
	return nil
}
