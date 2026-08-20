package keen

import (
	"reflect"
	"testing"
)

func TestSort(t *testing.T) {
	tests := []struct {
		name string
		in   []Repository
		want []Repository
	}{
		{
			name: "empty slice",
			in:   []Repository{},
			want: []Repository{},
		},
		{
			name: "single repository",
			in:   []Repository{{Name: "a", Path: "/a", Dirty: false}},
			want: []Repository{{Name: "a", Path: "/a", Dirty: false}},
		},
		{
			name: "clean before dirty",
			in: []Repository{
				{Name: "Z-Dirty", Path: "/z", Dirty: true},
				{Name: "A-Clean", Path: "/a", Dirty: false},
			},
			want: []Repository{
				{Name: "A-Clean", Path: "/a", Dirty: false},
				{Name: "Z-Dirty", Path: "/z", Dirty: true},
			},
		},
		{
			name: "alphabetical name sorting within groups",
			in: []Repository{
				{Name: "Zeta", Path: "/zeta", Dirty: false},
				{Name: "Alpha", Path: "/alpha", Dirty: false},
				{Name: "Delta", Path: "/delta", Dirty: false},
				{Name: "Omega", Path: "/omega", Dirty: true},
				{Name: "Beta", Path: "/beta", Dirty: true},
			},
			want: []Repository{
				{Name: "Alpha", Path: "/alpha", Dirty: false},
				{Name: "Delta", Path: "/delta", Dirty: false},
				{Name: "Zeta", Path: "/zeta", Dirty: false},
				{Name: "Beta", Path: "/beta", Dirty: true},
				{Name: "Omega", Path: "/omega", Dirty: true},
			},
		},
		{
			name: "path tie-breaker when names are equal",
			in: []Repository{
				{Name: "Common", Path: "/b-path", Dirty: false},
				{Name: "Common", Path: "/a-path", Dirty: false},
				{Name: "Common", Path: "/c-path", Dirty: true},
				{Name: "Common", Path: "/aa-path", Dirty: true},
			},
			want: []Repository{
				{Name: "Common", Path: "/a-path", Dirty: false},
				{Name: "Common", Path: "/b-path", Dirty: false},
				{Name: "Common", Path: "/aa-path", Dirty: true},
				{Name: "Common", Path: "/c-path", Dirty: true},
			},
		},
		{
			name: "mixed complete ordering",
			in: []Repository{
				{Name: "Z-Dirty", Path: "/z", Dirty: true},
				{Name: "B-Clean", Path: "/b", Dirty: false},
				{Name: "A-Dirty", Path: "/a2", Dirty: true},
				{Name: "A-Clean", Path: "/a1", Dirty: false},
				{Name: "C-Clean", Path: "/c", Dirty: false},
				{Name: "C-Dirty", Path: "/c2", Dirty: true},
			},
			want: []Repository{
				{Name: "A-Clean", Path: "/a1", Dirty: false},
				{Name: "B-Clean", Path: "/b", Dirty: false},
				{Name: "C-Clean", Path: "/c", Dirty: false},
				{Name: "A-Dirty", Path: "/a2", Dirty: true},
				{Name: "C-Dirty", Path: "/c2", Dirty: true},
				{Name: "Z-Dirty", Path: "/z", Dirty: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]Repository, len(tt.in))
			copy(got, tt.in)
			Sort(got)
			if len(got) != len(tt.want) {
				t.Fatalf("Sort() got len %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Name != tt.want[i].Name || got[i].Path != tt.want[i].Path || got[i].Dirty != tt.want[i].Dirty {
					t.Errorf("Sort() at index %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFilter(t *testing.T) {
	repos := []Repository{
		{Name: "clean-1", Dirty: false},
		{Name: "dirty-1", Dirty: true},
		{Name: "clean-2", Dirty: false},
	}

	tests := []struct {
		name string
		opts Options
		want []Repository
	}{
		{
			name: "no flags returns all",
			opts: Options{ShowClean: false, ShowDirty: false},
			want: []Repository{
				{Name: "clean-1", Dirty: false},
				{Name: "dirty-1", Dirty: true},
				{Name: "clean-2", Dirty: false},
			},
		},
		{
			name: "clean only",
			opts: Options{ShowClean: true, ShowDirty: false},
			want: []Repository{
				{Name: "clean-1", Dirty: false},
				{Name: "clean-2", Dirty: false},
			},
		},
		{
			name: "dirty only",
			opts: Options{ShowClean: false, ShowDirty: true},
			want: []Repository{
				{Name: "dirty-1", Dirty: true},
			},
		},
		{
			name: "both flags returns all",
			opts: Options{ShowClean: true, ShowDirty: true},
			want: []Repository{
				{Name: "clean-1", Dirty: false},
				{Name: "dirty-1", Dirty: true},
				{Name: "clean-2", Dirty: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter(repos, tt.opts)
			if len(got) != len(tt.want) {
				t.Fatalf("Filter() got %d repositories, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Name != tt.want[i].Name || got[i].Dirty != tt.want[i].Dirty {
					t.Errorf("Filter() at index %d = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}

	t.Run("zero matches returns empty", func(t *testing.T) {
		emptyRepos := []Repository{{Name: "clean-1", Dirty: false}}
		got := Filter(emptyRepos, Options{ShowClean: false, ShowDirty: true})
		if len(got) != 0 {
			t.Errorf("expected 0 repos, got %d", len(got))
		}
	})

	t.Run("does not mutate source slice", func(t *testing.T) {
		src := []Repository{{Name: "clean-1", Dirty: false}, {Name: "dirty-1", Dirty: true}}
		srcCopy := make([]Repository, len(src))
		copy(srcCopy, src)
		_ = Filter(src, Options{ShowClean: true, ShowDirty: false})
		if !reflect.DeepEqual(src, srcCopy) {
			t.Errorf("Filter mutated source slice: got %v, want %v", src, srcCopy)
		}
	})
}
