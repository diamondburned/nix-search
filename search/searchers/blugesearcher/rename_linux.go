//go:build !darwin
// +build !darwin

package blugesearcher

import (
	"os"

	"golang.org/x/sys/unix"
)

func Rename(dir *os.File, oldname, newname string) error {
    return unix.Renameat2(int(dir.Fd()), oldname, int(dir.Fd()), newname, unix.RENAME_EXCHANGE)
}
