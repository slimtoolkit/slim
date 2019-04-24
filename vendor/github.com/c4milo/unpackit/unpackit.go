// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package unpackit allows you to easily unpack *.tar.gz, *.tar.bzip2, *.tar.xz, *.zip and *.tar files.
// There are not CGO involved nor hard dependencies of any type.
package unpackit

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dsnet/compress/bzip2"
	gzip "github.com/klauspost/pgzip"
	"github.com/pkg/errors"
	"github.com/ulikunitz/xz"
)

var (
	magicZIP  = []byte{0x50, 0x4b, 0x03, 0x04}
	magicGZ   = []byte{0x1f, 0x8b}
	magicBZIP = []byte{0x42, 0x5a}
	magicTAR  = []byte{0x75, 0x73, 0x74, 0x61, 0x72} // at offset 257
	magicXZ   = []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}
)

// Check whether a file has the magic number for tar, gzip, bzip2 or zip files
//
// Note that this function does not advance the Reader.
//
// 50 4b 03 04 for pkzip format
// 1f 8b for .gz format
// 42 5a for .bzip format
// 75 73 74 61 72 at offset 257 for tar files
// fd 37 7a 58 5a 00 for .xz format
func magicNumber(reader *bufio.Reader, offset int) (string, error) {
	headerBytes, err := reader.Peek(offset + 6)
	if err != nil {
		return "", err
	}

	magic := headerBytes[offset : offset+6]

	if bytes.Equal(magicTAR, magic[0:5]) {
		return "tar", nil
	}

	if bytes.Equal(magicZIP, magic[0:4]) {
		return "zip", nil
	}

	if bytes.Equal(magicGZ, magic[0:2]) {
		return "gzip", nil
	} else if bytes.Equal(magicBZIP, magic[0:2]) {
		return "bzip", nil
	}

	if bytes.Equal(magicXZ, magic) {
		return "xz", nil
	}

	return "", nil
}

// Unpack unpacks a compressed stream. Magic numbers are used to determine what
// decompressor and/or unarchiver to use.
func Unpack(reader io.Reader, destPath string) (string, error) {
	var err error
	if destPath == "" {
		destPath, err = ioutil.TempDir(os.TempDir(), "unpackit-")
		if err != nil {
			return "", err
		}
	}

	// Makes sure destPath exists
	if err := os.MkdirAll(destPath, 0740); err != nil {
		return "", err
	}

	r := bufio.NewReader(reader)

	// Reads magic number from the stream so we can better determine how to proceed
	ftype, err := magicNumber(r, 0)
	if err != nil {
		return "", err
	}

	var decompressingReader *bufio.Reader
	switch ftype {
	case "gzip":
		gzr, err := gzip.NewReader(r)
		if err != nil {
			return "", err
		}

		defer func() {
			if err := gzr.Close(); err != nil {
				fmt.Printf("%+v", errors.Wrapf(err, "unpackit: failed closing gzip reader"))
			}
		}()

		decompressingReader = bufio.NewReader(gzr)
	case "xz":
		xzr, err := xz.NewReader(r)
		if err != nil {
			return "", err
		}

		decompressingReader = bufio.NewReader(xzr)
	case "bzip":
		br, err := bzip2.NewReader(r, nil)
		if err != nil {
			return "", err
		}

		defer func() {
			if err := br.Close(); err != nil {
				fmt.Printf("%+v", errors.Wrapf(err, "unpackit: failed closing bzip2 reader"))
			}
		}()

		decompressingReader = bufio.NewReader(br)
	case "zip":
		// Like TAR, ZIP is also an archiving format, therefore we can just return
		// after it finishes
		return Unzip(r, destPath)
	default:
		// maybe it is a tarball file
		decompressingReader = r
	}

	// Check magic number in offset 257 too see if this is also a TAR file
	ftype, err = magicNumber(decompressingReader, 257)
	if err != nil {
		return "", err
	}
	if ftype == "tar" {
		return Untar(decompressingReader, destPath)
	}

	// If it's not a TAR archive then save it to disk as is.
	destRawFile := filepath.Join(destPath, sanitize(path.Base("unknown-pack")))

	// Creates destination file
	destFile, err := os.Create(destRawFile)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			log.Println(err)
		}
	}()

	// Copies data to destination file
	if _, err := io.Copy(destFile, decompressingReader); err != nil {
		return "", err
	}

	return destPath, nil
}

// Unzip unpacks a ZIP stream. When given a os.File reader it will get its size without
// reading the entire zip file in memory.
func Unzip(r io.Reader, destPath string) (string, error) {
	var (
		zr  *zip.Reader
		err error
	)

	if f, ok := r.(*os.File); ok {
		fstat, err := f.Stat()
		if err != nil {
			return "", err
		}
		zr, err = zip.NewReader(f, fstat.Size())
	} else {
		data, err := ioutil.ReadAll(r)
		if err != nil {
			return "", err
		}
		memReader := bytes.NewReader(data)
		zr, err = zip.NewReader(memReader, memReader.Size())
	}

	if err != nil {
		return "", err
	}

	return unpackZip(zr, destPath)
}

func unpackZip(zr *zip.Reader, destPath string) (string, error) {
	for _, f := range zr.File {
		err := unzipFile(f, destPath)
		if err != nil {
			return "", err
		}
	}
	return destPath, nil
}

func unzipFile(f *zip.File, destPath string) error {
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filepath.Join(destPath, f.Name), f.Mode().Perm()); err != nil {
			return err
		}
		return nil
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		if err := rc.Close(); err != nil {
			log.Println(err)
		}
	}()

	filePath := sanitize(f.Name)
	destPath = filepath.Join(destPath, filePath)

	// If directories were not included in the archive but are part of the file name,
	// we create them relative to the destination path.
	fileDir := filepath.Dir(destPath)
	_, err = os.Lstat(fileDir)
	if err != nil {
		if err := os.MkdirAll(fileDir, 0700); err != nil {
			return err
		}
	}

	file, err := os.Create(destPath)
	if err != nil {
		return err
	}

	defer func() {
		if err := file.Close(); err != nil {
			log.Println(err)
		}
	}()

	if err := file.Chmod(f.Mode()); err != nil {
		log.Printf("warn: failed setting file permissions for %q: %#v", file.Name(), err)
	}

	if err := os.Chtimes(file.Name(), time.Now(), f.ModTime()); err != nil {
		log.Printf("warn: failed setting file atime and mtime for %q: %#v", file.Name(), err)
	}

	if _, err := io.CopyN(file, rc, int64(f.UncompressedSize64)); err != nil {
		return err
	}

	return nil
}

// Untar unarchives a TAR archive and returns the final destination path or an error
func Untar(data io.Reader, destPath string) (string, error) {
	// Makes sure destPath exists
	if err := os.MkdirAll(destPath, 0740); err != nil {
		return "", err
	}

	tr := tar.NewReader(data)

	// Iterate through the files in the archive.
	rootdir := destPath
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}

		if err != nil {
			return rootdir, err
		}

		// Skip pax_global_header with the commit ID this archive was created from
		if hdr.Name == "pax_global_header" {
			continue
		}

		fp := filepath.Join(destPath, sanitize(hdr.Name))
		if hdr.FileInfo().IsDir() {
			if rootdir == destPath {
				rootdir = fp
			}

			if err := os.MkdirAll(fp, os.FileMode(hdr.Mode)); err != nil {
				return rootdir, err
			}
			continue
		}

		_, untarErr := untarFile(hdr, tr, fp, rootdir)
		if untarErr != nil {
			return rootdir, untarErr
		}
	}

	return rootdir, nil
}

func untarFile(hdr *tar.Header, tr *tar.Reader, fp, rootdir string) (string, error) {
	parentDir, _ := filepath.Split(fp)

	if err := os.MkdirAll(parentDir, 0740); err != nil {
		return rootdir, err
	}

	file, err := os.Create(fp)
	if err != nil {
		return rootdir, err
	}

	defer func() {
		if err := file.Close(); err != nil {
			log.Println(err)
		}
	}()

	if err := file.Chmod(os.FileMode(hdr.Mode)); err != nil {
		log.Printf("warn: failed setting file permissions for %q: %#v", file.Name(), err)
	}

	if err := os.Chtimes(file.Name(), time.Now(), hdr.ModTime); err != nil {
		log.Printf("warn: failed setting file atime and mtime for %q: %#v", file.Name(), err)
	}

	if _, err := io.Copy(file, tr); err != nil {
		return rootdir, err
	}

	return rootdir, nil
}

// Sanitizes name to avoid overwriting sensitive system files when unarchiving
func sanitize(name string) string {
	// Gets rid of volume drive label in Windows
	if len(name) > 1 && name[1] == ':' && runtime.GOOS == "windows" {
		name = name[2:]
	}

	name = filepath.Clean(name)
	name = filepath.ToSlash(name)
	for strings.HasPrefix(name, "../") {
		name = name[3:]
	}

	return name
}
