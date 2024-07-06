package search

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
)

// Derivation is either a package or a package set.
type Derivation interface {
	isDerivation()
}

// Package is a package that is a derivation.
type Package struct {
	Name                string   `json:"name,omitempty"`
	Version             string   `json:"version,omitempty"`
	Description         string   `json:"description"`
	LongDescription     string   `json:"longDescription,omitempty"`
	Licenses            []string `json:"license,omitempty"` // usually SPDX identifiers
	MainProgram         string   `json:"mainProgram,omitempty"`
	Broken              bool     `json:"broken,omitempty"`
	Unfree              bool     `json:"unfree,omitempty"`
	UnsupportedPlatform bool     `json:"unsupportedPlatform,omitempty"`

	// Homepages           []string `json:"homepages,omitempty"`
}

// TopLevelPackages is a set of packages that are top-level packages.
type TopLevelPackages struct {
	PackageSet
	// Channel is the name of the channel that these packages are from.
	// For example, "nixpkgs".
	Channel string `json:"channel"`
}

// Walk walks the package set, calling f on each derivation. If f returns
// false, the walk is stopped. A DFS is used.
func (s TopLevelPackages) Walk(f func(Path, Package) bool) {
	s.PackageSet.Walk(Path{s.Channel}, f)
}

// PackageSet is a package that is a package set.
type PackageSet map[string]Derivation

// Walk walks the package set, calling f on each derivation. If f returns
// false, the walk is stopped. A DFS is used.
func (s PackageSet) Walk(selfPath Path, f func(Path, Package) bool) {
	type node struct {
		path Path
		pkgs PackageSet
	}

	stack := make([]node, 1, len(s))
	stack[0] = node{selfPath, s}

	for len(stack) > 0 {
		// Pop the top of the stack.
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		for name, v := range top.pkgs {
			switch v := v.(type) {
			case Package:
				if !f(top.path.PushInplace(name), v) {
					return
				}
			case PackageSet:
				stack = append(stack, node{
					path: top.path.Push(name),
					pkgs: v,
				})
			default:
				panic("unknown package type")
			}
		}
	}
}

// Count returns the number of packages in this set.
func (s PackageSet) Count() int {
	var count int
	s.Walk(Path{}, func(Path, Package) bool {
		count++
		return true
	})
	return count
}

// MarshalJSON implements json.Marshaler.
func (s PackageSet) MarshalJSON() ([]byte, error) {
	// Marshal this set in a special way: we want to marshal it as an object
	// with an extra type field, so that we can distinguish between a package
	// set and a derivation.
	m := make(map[string]any, len(s))
	for k, v := range s {
		switch v := v.(type) {
		case Package:
			v.Name = ""
			m[k] = struct {
				Type string `json:"_type"`
				Package
			}{
				Type:    "derivation",
				Package: v,
			}
		case PackageSet:
			m[k] = struct {
				Type string `json:"_type"`
				PackageSet
			}{
				Type:       "packageSet",
				PackageSet: v,
			}
		default:
			panic("unknown package type")
		}
	}

	return json.Marshal(m)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s PackageSet) UnmarshalJSON(b []byte) error {
	raws := make(map[string]json.RawMessage)
	if err := json.Unmarshal(b, &raws); err != nil {
		return err
	}

	for k, v := range raws {
		var t struct {
			Type string `json:"_type"`
		}
		if err := json.Unmarshal(v, &t); err != nil {
			return err
		}

		switch t.Type {
		case "derivation":
			var drv Package
			if err := json.Unmarshal(v, &drv); err != nil {
				return err
			}
			drv.Name = k
			s[k] = drv

		case "set":
			var set PackageSet
			if err := json.Unmarshal(v, &set); err != nil {
				return err
			}
			s[k] = set

		default:
			return fmt.Errorf("unknown package type %q", t)
		}
	}

	return nil
}

func (Package) isDerivation()    {}
func (PackageSet) isDerivation() {}

// IndexPackagesOpts are options for IndexPackages.
type IndexPackagesOpts struct {
	// Nixpkgs is the Nixpkgs path to index.
	Nixpkgs string
	// Parallelism is the number of parallel workers to use.
	Parallelism int
}

// DefaultIndexPackageOpts are the default options for IndexPackages.
var DefaultIndexPackageOpts = IndexPackagesOpts{
	Nixpkgs:     "<nixpkgs>",
	Parallelism: runtime.GOMAXPROCS(-1),
}

// IndexPackages indexes all packages in the given channel.
func IndexPackages(ctx context.Context, opts IndexPackagesOpts) (TopLevelPackages, error) {
	ctx = hclog.WithContext(ctx,
		hclog.FromContext(ctx).Named("search.IndexPackages"))

	logger := hclog.FromContext(ctx)
	logger.Debug(
		"indexing packages",
		"nixpkgs", opts.Nixpkgs,
		"parallelism", opts.Parallelism)

	pi := newPackageIndexer(opts)

	name := opts.Nixpkgs
	if strings.HasPrefix(name, "<") && strings.HasSuffix(name, ">") {
		name = name[1 : len(name)-1]
	} else {
		name = path.Base(name)
	}

	return TopLevelPackages{
		PackageSet: pi.packages,
		Channel:    name,
	}, pi.start(ctx)
}

// Path is a path to a package. It always starts with the channel name.
type Path []string

// FromDotPath converts a dot-separated path to a Path.
func FromDotPath(path string) Path {
	return strings.Split(path, ".")
}

// String implements fmt.Stringer.
func (p Path) String() string {
	return strings.Join(p, ".")
}

// Push appends names to the path. The returned path will be a reallocated
// slice.
func (p Path) Push(names ...string) Path {
	return append(append([]string(nil), p...), names...)
}

// PushInplace is like Append, but it appends to the path in-place.
// Go may or may not reallocate the slice.
func (p Path) PushInplace(names ...string) Path {
	return append(p, names...)
}

// Pop pops the last name off the path. The returned path will not be a
// reallocated slice.
func (p Path) Pop() Path {
	return p[:len(p)-1]
}

// Clone clones the path.
func (p Path) Clone() Path {
	return append([]string(nil), p...)
}

type packageIndexJob struct {
	attrs  []string
	parent PackageSet
}

type packageIndexResult struct {
	packageIndexJob
	error error
	jobs  []packageIndexJob // more jobs
}

func errorPackageIndexResult(job packageIndexJob, err error) packageIndexResult {
	return packageIndexResult{
		packageIndexJob: job,
		error:           err,
	}
}

type packageIndexer struct {
	opts     IndexPackagesOpts
	packages PackageSet
}

func newPackageIndexer(opts IndexPackagesOpts) packageIndexer {
	return packageIndexer{
		packages: PackageSet{},
		opts:     opts,
	}
}

func (pi packageIndexer) start(ctx context.Context) error {
	logger := hclog.FromContext(ctx)
	defer logger.Debug("done indexing packages")

	var wg sync.WaitGroup
	defer wg.Wait()

	jobs := make([]packageIndexJob, 1, 1024)
	jobs[0] = packageIndexJob{
		attrs:  []string{},
		parent: pi.packages,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCh := make(chan packageIndexJob)
	outCh := make(chan packageIndexResult)

	for i := 0; i < pi.opts.Parallelism; i++ {
		wg.Add(1)
		go func() {
			pi.worker(ctx, jobCh, outCh)
			wg.Done()
		}()
	}

	// ongoing keeps track of the number of queued jobs. It is only decremented
	// when a job is finished, so it is not a count of the number of jobs that
	// have been started (which is len(jobQueue)).
	var ongoing int

	for len(jobs) > 0 || ongoing > 0 {
		var job packageIndexJob
		var jobCh2 chan<- packageIndexJob

		if len(jobs) > 0 {
			job = jobs[0]
			jobCh2 = jobCh
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case jobCh2 <- job:
			jobs = jobs[1:]
			ongoing++

			logger.Debug("queued job", "attrs", job.attrs)

		case result := <-outCh:
			ongoing--

			level := hclog.Debug
			msg := "finished job"
			if result.error != nil {
				if len(result.attrs) == 0 {
					return result.error
				}
				level = hclog.Warn
				msg = "failed job"
			}

			logger.Log(level, msg,
				"attrs", result.attrs,
				"error", result.error,
				"jobs", len(result.jobs))

			jobs = append(jobs, result.jobs...)
		}
	}

	return nil
}

func (pi packageIndexer) worker(ctx context.Context, jobCh <-chan packageIndexJob, outCh chan<- packageIndexResult) {
	emit := func(out packageIndexResult) {
		select {
		case <-ctx.Done():
			log := hclog.FromContext(ctx)
			log.Warn("worker exiting early due to context cancellation")
			return
		case outCh <- out:
			// ok
		}
	}

	for {
		select {
		case <-ctx.Done():
			return

		case job := <-jobCh:
			log := hclog.FromContext(ctx)
			log.Debug("worker: indexing", "attrs", strings.Join(job.attrs, "."))

			out, err := dumpPackages(ctx, pi.opts.Nixpkgs, job.attrs)
			if err != nil {
				emit(errorPackageIndexResult(job, err))
				continue
			}

			var jobs []packageIndexJob

			for attr, pkg := range out {
				if pkg.HasMore {
					newSet := PackageSet{}
					job.parent[attr] = newSet

					jobs = append(jobs, packageIndexJob{
						attrs:  appendCopy(job.attrs, attr),
						parent: newSet,
					})
					continue
				}

				ppkg := Package{Name: attr}
				if err := json.Unmarshal(pkg.Meta, &ppkg); err != nil {
					err = fmt.Errorf("cannot unmarshal package %q: %w", attr, err)
					emit(errorPackageIndexResult(job, err))
					continue
				}

				job.parent[attr] = ppkg
			}

			emit(packageIndexResult{
				packageIndexJob: job,
				jobs:            jobs,
			})
		}
	}
}

func appendCopy(dst []string, src ...string) []string {
	return append(append([]string(nil), dst...), src...)
}

func max(i, j int) int {
	if i > j {
		return i
	}
	return j
}
