package bluge

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/blugelabs/bluge"
	"github.com/blugelabs/bluge/index"
	"libdb.so/nix-search/search"
)

func batchPackageSet(packages search.TopLevelPackages) (*index.Batch, error) {
	batch := bluge.NewBatch()
	packages.Walk(func(path search.Path, drv search.Package) bool {
		doc := newPackageDocument(path, drv)
		batch.Update(doc.ID(), doc)
		return true
	})
	return batch, nil
}

func newPackageDocument(path search.Path, pkg search.Package) *bluge.Document {
	drvJSON, err := json.Marshal(pkg)
	if err != nil {
		log.Panicln("cannot marshal derivation:", err)
	}

	log.Printf("indexing %s: %#v", path.String(), pkg)

	doc := bluge.NewDocument(path.String())
	doc.AddField(bluge.NewStoredOnlyField("json", drvJSON))
	doc.AddField(bluge.NewTextField("name", pkg.Name))
	doc.AddField(bluge.NewTextField("description", pkg.Description))

	return doc
}

func defaultPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot get user cache dir: %w", err)
	}

	cacheDir = filepath.Join(cacheDir, "nix-search")
	return cacheDir, nil
}
