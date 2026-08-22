package keen

import (
	"path/filepath"
	"reflect"
	"testing"
)

func repo(path string) Repository {
	return Repository{Name: filepath.Base(path), Path: path}
}

func repos(paths ...string) []Repository {
	out := make([]Repository, 0, len(paths))
	for _, p := range paths {
		out = append(out, repo(p))
	}
	return out
}

func names(in []Repository) []string {
	out := make([]string, len(in))
	for i := range in {
		out[i] = in[i].Name
	}
	return out
}

// TestResolveUniqueNamesUnchanged: repositories whose basenames are unique
// within the set never expand (S1, S12).
func TestResolveUniqueNamesUnchanged(t *testing.T) {
	in := repos("/site/keen", "/home/web", "/x/api")
	got := ResolveDisplayIdentities(in)
	want := []string{"keen", "web", "api"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("names = %v, want %v", names(got), want)
	}
}

// TestResolveSiblingCollision: minimum one-level expansion (S2, contract #2).
func TestResolveSiblingCollision(t *testing.T) {
	got := ResolveDisplayIdentities(repos("/work/api", "/personal/api"))
	want := []string{"work/api", "personal/api"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("names = %v, want %v", names(got), want)
	}
}

// TestResolveSharedParentCollision: full-candidate comparison across rounds;
// one ancestor is not always sufficient (S3, contract #3).
func TestResolveSharedParentCollision(t *testing.T) {
	got := ResolveDisplayIdentities(repos("/foo/work/api", "/bar/work/api"))
	want := []string{"foo/work/api", "bar/work/api"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("names = %v, want %v", names(got), want)
	}
}

// TestResolveAsymmetricDepth: unequal depths must not assume the shortest
// path bounds expansion (S4, contract #4).
func TestResolveAsymmetricDepth(t *testing.T) {
	got := ResolveDisplayIdentities(repos("/p/a/api", "/q/b/a/api"))
	want := []string{"p/a/api", "b/a/api"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("names = %v, want %v", names(got), want)
	}
}

// TestResolveTargetedGrowth: only members of an unresolved class expand;
// n/api must stay unperturbed even though its round-1 candidate matched
// another member's intermediate candidate (S5, contract #5).
func TestResolveTargetedGrowth(t *testing.T) {
	got := ResolveDisplayIdentities(repos("/m/api", "/n/api", "/o/m/api"))
	want := []string{"m/api", "n/api", "o/m/api"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("names = %v, want %v", names(got), want)
	}
}

// TestResolveThreeWayMixedDepths: equal-depth, deeper, and shared-component
// collisions resolve with only necessary expansion (S6, contract #6).
func TestResolveThreeWayMixedDepths(t *testing.T) {
	got := ResolveDisplayIdentities(repos("/a/x/api", "/b/x/api", "/a/y/deep/api"))
	want := []string{"a/x/api", "b/x/api", "deep/api"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("names = %v, want %v", names(got), want)
	}
}

// TestResolveSingleSurvivorKeepsBasename: a collision that filtering reduced
// to one member collapses back to the plain basename (S7, contract #7).
func TestResolveSingleSurvivorKeepsBasename(t *testing.T) {
	in := repos("/personal/api")
	got := ResolveDisplayIdentities(in)
	if got[0].Name != "api" {
		t.Errorf("name = %q, want api", got[0].Name)
	}
}

// TestResolveUnrelatedImmunity: expanded identities contain a separator and
// therefore cannot shadow or disturb unrelated basenames (S7 immunity,
// contract #8).
func TestResolveUnrelatedImmunity(t *testing.T) {
	in := repos("/work/api", "/personal/api", "/site/projects/web", "/elsewhere/keen")
	got := ResolveDisplayIdentities(in)
	want := []string{"work/api", "personal/api", "web", "keen"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("names = %v, want %v", names(got), want)
	}
}

// TestResolveEmptyInput (contract #9).
func TestResolveEmptyInput(t *testing.T) {
	if got := ResolveDisplayIdentities([]Repository{}); len(got) != 0 {
		t.Errorf("expected empty result, got %d entries", len(got))
	}
}

// TestResolveSingleRepository (contract #10).
func TestResolveSingleRepository(t *testing.T) {
	got := ResolveDisplayIdentities(repos("/anywhere/solitary"))
	if got[0].Name != "solitary" {
		t.Errorf("name = %q, want solitary", got[0].Name)
	}
}

// TestResolveDeterministic: repeated calls produce identical results
// (S9, contract #11).
func TestResolveDeterministic(t *testing.T) {
	in := repos("/work/api", "/personal/api", "/a/y/deep/api", "/b/x/api", "/a/x/api")
	first := ResolveDisplayIdentities(in)
	for range 5 {
		if again := ResolveDisplayIdentities(in); !reflect.DeepEqual(first, again) {
			t.Fatalf("non-deterministic resolution")
		}
	}
}

// TestResolveInputImmutability: source slice and elements are unchanged
// (contract #12).
func TestResolveInputImmutability(t *testing.T) {
	in := repos("/work/api", "/personal/api")
	before := repos("/work/api", "/personal/api")
	_ = ResolveDisplayIdentities(in)
	if !reflect.DeepEqual(in, before) {
		t.Errorf("input mutated: %+v", in)
	}
}

// TestResolveOrderPreservation: input order maps index-for-index to output
// order (contract #13).
func TestResolveOrderPreservation(t *testing.T) {
	in := repos("/zzz/unique-b", "/work/api", "/aaa/unique-a", "/personal/api")
	got := ResolveDisplayIdentities(in)
	for i := range in {
		if got[i].Path != in[i].Path {
			t.Errorf("output[%d] has Path %q, want %q", i, got[i].Path, in[i].Path)
		}
	}
	wantNames := []string{"unique-b", "work/api", "unique-a", "personal/api"}
	if !reflect.DeepEqual(names(got), wantNames) {
		t.Errorf("names = %v, want %v", names(got), wantNames)
	}
}

// TestResolveDuplicatePathsTerminate: identical paths cannot be separated by
// expansion; the resolver must freeze instead of looping forever
// (contract #14).
func TestResolveDuplicatePathsTerminate(t *testing.T) {
	in := []Repository{repo("/dup/api"), repo("/dup/api")}
	got := ResolveDisplayIdentities(in)
	if got[0].Name != got[1].Name {
		t.Errorf("duplicate paths resolved differently: %q vs %q", got[0].Name, got[1].Name)
	}
}

// TestResolveCaseSensitive: exact-byte comparison keeps distinct casings
// collision-free (contract #15).
func TestResolveCaseSensitive(t *testing.T) {
	got := ResolveDisplayIdentities(repos("/one/Foo", "/two/foo"))
	want := []string{"Foo", "foo"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("names = %v, want %v", names(got), want)
	}
}

// TestResolveAfterFilterCollapsesBack pins the pipeline-stage contract:
// resolution runs on the displayed set, so a filter-reduced survivor returns
// to its basename (S7 end-to-end through Sort/Filter).
func TestResolveAfterFilterCollapsesBack(t *testing.T) {
	in := repos("/work/api", "/personal/api")
	Sort(in)

	all := ResolveDisplayIdentities(Filter(in, Options{}))
	// Canonical sort: equal basenames tie-break on Path, so /personal/api
	// precedes /work/api. Resolution must not disturb that order.
	wantAll := []string{"personal/api", "work/api"}
	if !reflect.DeepEqual(names(all), wantAll) {
		t.Fatalf("all-mode names = %v, want %v", names(all), wantAll)
	}

	dirtyOnly := Filter(in, Options{ShowDirty: true})
	resolved := ResolveDisplayIdentities(dirtyOnly)
	if len(resolved) == 1 && resolved[0].Name != "api" {
		t.Errorf("filtered survivor name = %q, want api", resolved[0].Name)
	}
}
