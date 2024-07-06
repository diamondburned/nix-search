package blugesearcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blugelabs/bluge"
	"github.com/hashicorp/go-hclog"
	"golang.org/x/sys/unix"
	"libdb.so/nix-search/search"
)

// PackagesIndexer implements search.PackagesIndexer.
type PackagesIndexer struct {
	writer *bluge.Writer
}

// IndexPackages indexes the given packages. If path is empty, the default
// path is used.
func IndexPackages(ctx context.Context, path string, packages search.TopLevelPackages) error {
	if path == "" {
		var err error

		path, err = DefaultIndexPath()
		if err != nil {
			return fmt.Errorf("cannot get default index path: %w", err)
		}
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("cannot create index directory: %w", err)
	}

	// Plan:
	// 1. Create a temporary directory which will be used as the new index.
	// 2. Index the packages into the new index.
	// 3. Swap the new index with the old index.
	// 4. Delete the old index (now the new index).

	batch, err := batchPackageSet(packages)
	if err != nil {
		return fmt.Errorf("cannot batch package set: %w", err)
	}

	newPath, err := os.MkdirTemp(path, "index-*")
	if err != nil {
		return fmt.Errorf("cannot create new index snapshot: %w", err)
	}
	defer os.RemoveAll(newPath)

	writer, err := bluge.OpenWriter(bluge.DefaultConfig(newPath))
	if err != nil {
		return fmt.Errorf("cannot open bluge writer: %w", err)
	}
	defer writer.Close()

	if err := writer.Batch(batch); err != nil {
		return fmt.Errorf("cannot batch index: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("cannot close index: %w", err)
	}

	if err := swapDir(path, "index", filepath.Base(newPath)); err != nil {
		return fmt.Errorf("cannot commit new index: %w", err)
	}

	if err := os.RemoveAll(newPath); err != nil {
		log := hclog.FromContext(ctx)
		log.Error("cannot remove new index snapshot", "path", newPath, "error", err)
	}

	return nil
}

// swapDir atomically swaps two directories.
func swapDir(basedir, oldname, newname string) error {
	dir, err := os.Open(basedir)
	if err != nil {
		return err
	}
	defer dir.Close()

	// Ensure that the old directory exists so we can swap it with the new
	// directory. Instances that spawn concurrently after this function may
	// open an empty directory, but that's fine, since the error would otherwise
	// be a not found error.
	if err := os.MkdirAll(filepath.Join(basedir, oldname), 0755); err != nil {
		return err
	}

	return unix.Renameat2(int(dir.Fd()), oldname, int(dir.Fd()), newname, unix.RENAME_EXCHANGE)
}
