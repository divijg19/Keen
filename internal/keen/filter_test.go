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
			name: "single clean",
			in:   []Repository{{Name: "a", Dirty: false}},
			want: []Repository{{Name: "a", Dirty: false}},
		},
		{
			name: "single dirty",
			in:   []Repository{{Name: "a", Dirty: true}},
			want: []Repository{{Name: "a", Dirty: true}},
		},
		{
			name: "clean before dirty with stability",
			in: []Repository{
				{Name: "dirty-1", Dirty: true},
				{Name: "clean-1", Dirty: false},
				{Name: "dirty-2", Dirty: true},
				{Name: "clean-2", Dirty: false},
				{Name: "clean-3", Dirty: false},
				{Name: "dirty-3", Dirty: true},
			},
			want: []Repository{
				{Name: "clean-1", Dirty: false},
				{Name: "clean-2", Dirty: false},
				{Name: "clean-3", Dirty: false},
				{Name: "dirty-1", Dirty: true},
				{Name: "dirty-2", Dirty: true},
				{Name: "dirty-3", Dirty: true},
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
				if got[i].Name != tt.want[i].Name || got[i].Dirty != tt.want[i].Dirty {
					t.Errorf("Sort() at index %d = %v, want %v", i, got[i], tt.want[i])
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
