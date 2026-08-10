// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

//go:build windows

package publisher

import "os"

func writeOwnerOnlyTestFile(root *os.Root, name string, contents []byte) error {
	if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
		return err
	}
	file, err := openOwnerOnlyFile(root, name)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
