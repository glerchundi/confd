package util

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/golang/glog"
)

// FileInfo describes a configuration file and is returned by fileStat.
type fileInfo struct {
	Uid  uint32
	Gid  uint32
	Mode os.FileMode
	Md5  string
}

// IsFileExist reports whether path exits.
func IsFileExist(fpath string) bool {
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		return false
	}
	return true
}

// IsSameConfig reports whether src and dest config files are equal.
// Two config files are equal when they have the same file contents and
// Unix permissions. The owner, group, and mode must match.
// It return false in other cases.
func IsSameConfig(src, dest string) (bool, error) {
	if !IsFileExist(dest) {
		return false, nil
	}
	dfi, err := getFileInfo(dest)
	if err != nil {
		return false, err
	}
	sfi, err := getFileInfo(src)
	if err != nil {
		return false, err
	}
	if dfi.Uid != sfi.Uid {
		glog.Infof("%s has UID %d should be %d", dest, dfi.Uid, sfi.Uid)
	}
	if dfi.Gid != sfi.Gid {
		glog.Infof("%s has GID %d should be %d", dest, dfi.Gid, sfi.Gid)
	}
	if dfi.Mode != sfi.Mode {
		glog.Infof("%s has mode %s should be %s", dest, os.FileMode(dfi.Mode), os.FileMode(sfi.Mode))
	}
	if dfi.Md5 != sfi.Md5 {
		glog.Infof("%s has md5sum %s should be %s", dest, dfi.Md5, sfi.Md5)
	}
	if dfi.Uid != sfi.Uid || dfi.Gid != sfi.Gid || dfi.Mode != sfi.Mode || dfi.Md5 != sfi.Md5 {
		return false, nil
	}
	return true, nil
}

// getFileInfo returns a FileInfo describing the named file.
func getFileInfo(name string) (fi fileInfo, err error) {
	if IsFileExist(name) {
		f, err := os.Open(name)
		if err != nil {
			return fi, err
		}
		defer f.Close()
		stats, _ := f.Stat()
		fi.Uid = stats.Sys().(*syscall.Stat_t).Uid
		fi.Gid = stats.Sys().(*syscall.Stat_t).Gid
		fi.Mode = stats.Mode()
		h := md5.New()
		io.Copy(h, f)
		fi.Md5 = fmt.Sprintf("%x", h.Sum(nil))
		return fi, nil
	} else {
		return fi, fmt.Errorf("File not found")
	}
}