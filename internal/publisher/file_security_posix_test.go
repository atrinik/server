// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

//go:build !windows

package publisher

import "os"

func writeOwnerOnlyTestFile(root *os.Root, name string, contents []byte) error {
	if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
		return err
	}
	return root.WriteFile(name, contents, 0o600)
}
