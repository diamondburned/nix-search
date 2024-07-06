package blugesearcher

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/blugelabs/bluge"
	"libdb.so/nix-search/search"

	blugeindex "github.com/blugelabs/bluge/index"
)

func batchPackageSet(packages search.TopLevelPackages) (*blugeindex.Batch, error) {
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

	doc := bluge.NewDocument(path.String())
	doc.AddField(bluge.NewStoredOnlyField("json", drvJSON))
	// hack because bluge is kinda balls and doesn't treat . as a word boundary
	doc.AddField(newField("path", strings.Join(path, " ")))
	doc.AddField(newField("name", pkg.Name))
	doc.AddField(newField("description", pkg.Description))

	return doc
}

// defaultIndexPath gets the default index path.
func defaultIndexPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot get user cache dir: %w", err)
	}

	cacheDir = filepath.Join(cacheDir, "nix-search")
	return cacheDir, nil
}

func newField(name string, value string) *bluge.TermField {
	return bluge.NewTextField(name, value).
		StoreValue().
		Aggregatable().
		HighlightMatches().
		SearchTermPositions()
}
