// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

//go:build windows

package publisher

import (
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsFileAllAccess windows.ACCESS_MASK = 0x001f01ff

func openOwnerOnlyFile(root *os.Root, name string) (*os.File, error) {
	parentName := path.Dir(name)
	baseName := path.Base(name)
	if baseName == "." || baseName == "/" {
		return nil, errors.New("owner-only file name is invalid")
	}
	if err := root.MkdirAll(parentName, 0o700); err != nil {
		return nil, err
	}
	parent, err := root.Open(parentName)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	parentInformation, err := parent.Stat()
	if err != nil || !parentInformation.IsDir() {
		return nil, errors.New("owner-only file parent is not a directory")
	}

	user, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	securityDescriptor, err := windows.SecurityDescriptorFromString(
		fmt.Sprintf("O:%sD:P(A;;FA;;;%s)", user.String(), user.String()),
	)
	if err != nil {
		return nil, err
	}
	objectName, err := windows.NewNTUnicodeString(baseName)
	if err != nil {
		return nil, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:             uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory:      windows.Handle(parent.Fd()),
		ObjectName:         objectName,
		Attributes:         windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
		SecurityDescriptor: securityDescriptor,
	}
	var handle windows.Handle
	err = windows.NtCreateFile(
		&handle,
		windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.READ_CONTROL,
		attributes,
		&windows.IO_STATUS_BLOCK{},
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN_IF,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT,
		0,
		0,
	)
	runtime.KeepAlive(securityDescriptor)
	runtime.KeepAlive(parent)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), name)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, errors.New("create owner-only file handle")
	}
	if err := validateOwnerOnlyFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func validateOwnerOnlyFile(file *os.File) error {
	user, err := currentUserSID()
	if err != nil {
		return err
	}
	descriptor, err := windows.GetSecurityInfo(
		windows.Handle(file.Fd()),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return err
	}
	owner, ownerDefaulted, err := descriptor.Owner()
	if err != nil || ownerDefaulted || owner == nil || !owner.Equals(user) {
		return errors.New("file owner is not the current user")
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("file DACL is not protected")
	}
	dacl, daclDefaulted, err := descriptor.DACL()
	if err != nil || daclDefaulted || dacl == nil || dacl.AceCount != 1 {
		return errors.New("file DACL is not current-user-only")
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil || ace == nil ||
		ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
		ace.Header.AceFlags&windows.INHERITED_ACE != 0 ||
		ace.Mask != windowsFileAllAccess {
		return errors.New("file DACL has an unsupported access entry")
	}
	aceUser := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !aceUser.IsValid() || !aceUser.Equals(user) {
		return errors.New("file DACL grants another identity")
	}
	return nil
}

func currentUserSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return nil, errors.New("read current Windows user")
	}
	return user.User.Sid.Copy()
}
