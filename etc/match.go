package etc

import (
	"path"
	"path/filepath"

	"github.com/renatopp/go-x/fsx"
)

// matches reports if the slash path `rel`, relative to Base, is matched by the
// rule patterns and not excluded.
func (r *Rule) matches(rel string) bool {
	if r.excludes(rel) {
		return false
	}
	for _, p := range r.Patterns {
		if fsx.ForceMatch(rel, p) {
			return true
		}
	}
	return false
}

// excludes reports if `rel` or any of its parent directories match an exclude
// pattern, so `-e node_modules` also ignores everything inside it.
func (r *Rule) excludes(rel string) bool {
	for p := rel; p != "." && p != "/" && p != ""; p = path.Dir(p) {
		for _, e := range r.Exclude {
			if fsx.ForceMatch(p, e) {
				return true
			}
		}
	}
	return false
}

// targetOf returns the target path for the file `rel`, joined with base.
func (r *Rule) targetOf(base, rel string) string {
	switch r.Target {
	case BaseTarget:
		return base
	case DirTarget:
		return filepath.Dir(filepath.Join(base, rel))
	default:
		return filepath.Join(base, rel)
	}
}
