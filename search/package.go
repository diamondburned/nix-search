package search

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
)

// Package is either a derivation or a package set.
type Package interface {
	isPackage()
}

// Derivation is a package that is a derivation.
type Derivation struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
	Broken      bool   `json:"broken,omitempty"`
}

// PackageSet is a package that is a package set.
type PackageSet map[string]Package

// MarshalJSON implements json.Marshaler.
func (s PackageSet) MarshalJSON() ([]byte, error) {
	// Marshal this set in a special way: we want to marshal it as an object
	// with an extra type field, so that we can distinguish between a package
	// set and a derivation.
	m := make(map[string]any, len(s))
	for k, v := range s {
		switch v := v.(type) {
		case Derivation:
			v.Name = ""
			m[k] = struct {
				Type string `json:"_type"`
				Derivation
			}{
				Type:       "derivation",
				Derivation: v,
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
			var drv Derivation
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

func (Derivation) isPackage() {}
func (PackageSet) isPackage() {}

// IndexPackagesOpts are options for IndexPackages.
type IndexPackagesOpts struct {
	Channel     string
	Parallelism int
}

// DefaultIndexPackageOpts are the default options for IndexPackages.
var DefaultIndexPackageOpts = IndexPackagesOpts{
	Channel:     "<nixpkgs>",
	Parallelism: runtime.GOMAXPROCS(-1),
}

// IndexPackages indexes all packages in the given channel.
func IndexPackages(ctx context.Context, opts IndexPackagesOpts) (PackageSet, error) {
	ctx = hclog.WithContext(ctx,
		hclog.FromContext(ctx).Named("search.IndexPackages"))

	pi := newPackageIndexer(opts)
	return pi.packages, pi.start(ctx)
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

			logger.Info("queued job", "attrs", job.attrs)

		case result := <-outCh:
			ongoing--

			logger.Info("finished job",
				"attrs", result.attrs,
				"error", result.error,
				"jobs", len(result.jobs))

			if len(result.attrs) == 0 && result.error != nil {
				return result.error
			}

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

			out, err := dumpPackages(ctx, pi.opts.Channel, job.attrs)
			if err != nil {
				emit(packageIndexResult{
					packageIndexJob: job,
					error:           err,
				})
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

				job.parent[attr] = Derivation{
					Name:        attr,
					Description: pkg.Description,
					Version:     pkg.Version,
					Broken:      pkg.Broken,
				}
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
