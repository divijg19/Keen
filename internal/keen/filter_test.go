package keen

import (
	"reflect"
	"testing"
	"time"
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

func TestParseRecentDuration(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"30s", 30 * time.Second, false},
		{"45m", 45 * time.Minute, false},
		{"24h", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"7D", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"2W", 14 * 24 * time.Hour, false},
		{"", 0, true},
		{"garbage", 0, true},
		{"7", 0, true},
		{"0d", 0, true},
		{"0h", 0, true},
		{"-3d", 0, true},
		{"-24h", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseRecentDuration(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRecentDuration(%q) error = %v, wantErr = %v", tt.in, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseRecentDuration(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	refTime := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	t1h := refTime.Add(-1 * time.Hour)
	t2d := refTime.Add(-2 * 24 * time.Hour)
	t8d := refTime.Add(-8 * 24 * time.Hour)
	t7dCutoff := refTime.Add(-7 * 24 * time.Hour) // exactly 7d ago

	repos := []Repository{
		{Name: "clean-1h", Dirty: false, LastCommitAt: t1h},
		{Name: "clean-8d", Dirty: false, LastCommitAt: t8d},
		{Name: "clean-no-commits", Dirty: false, LastCommitAt: time.Time{}},
		{Name: "dirty-2d", Dirty: true, LastCommitAt: t2d},
		{Name: "dirty-7d-exact", Dirty: true, LastCommitAt: t7dCutoff},
		{Name: "dirty-8d", Dirty: true, LastCommitAt: t8d},
	}

	tests := []struct {
		name string
		opts Options
		want []string // matching repository names
	}{
		{
			name: "no flags returns all",
			opts: Options{ShowClean: false, ShowDirty: false},
			want: []string{"clean-1h", "clean-8d", "clean-no-commits", "dirty-2d", "dirty-7d-exact", "dirty-8d"},
		},
		{
			name: "clean only",
			opts: Options{ShowClean: true, ShowDirty: false},
			want: []string{"clean-1h", "clean-8d", "clean-no-commits"},
		},
		{
			name: "dirty only",
			opts: Options{ShowClean: false, ShowDirty: true},
			want: []string{"dirty-2d", "dirty-7d-exact", "dirty-8d"},
		},
		{
			name: "both flags returns all",
			opts: Options{ShowClean: true, ShowDirty: true},
			want: []string{"clean-1h", "clean-8d", "clean-no-commits", "dirty-2d", "dirty-7d-exact", "dirty-8d"},
		},
		{
			name: "recent only (7d)",
			opts: Options{HasRecent: true, RecentAfter: t7dCutoff},
			// includes clean-1h, dirty-2d, dirty-7d-exact (exact cutoff matches >=)
			want: []string{"clean-1h", "dirty-2d", "dirty-7d-exact"},
		},
		{
			name: "clean AND recent (7d)",
			opts: Options{ShowClean: true, HasRecent: true, RecentAfter: t7dCutoff},
			want: []string{"clean-1h"},
		},
		{
			name: "dirty AND recent (7d)",
			opts: Options{ShowDirty: true, HasRecent: true, RecentAfter: t7dCutoff},
			want: []string{"dirty-2d", "dirty-7d-exact"},
		},
		{
			name: "both flags AND recent (7d)",
			opts: Options{ShowClean: true, ShowDirty: true, HasRecent: true, RecentAfter: t7dCutoff},
			want: []string{"clean-1h", "dirty-2d", "dirty-7d-exact"},
		},
		{
			name: "recent (30m) - zero matches",
			opts: Options{HasRecent: true, RecentAfter: refTime.Add(-30 * time.Minute)},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter(repos, tt.opts)
			gotNames := make([]string, len(got))
			for i, r := range got {
				gotNames[i] = r.Name
			}
			if !reflect.DeepEqual(gotNames, tt.want) {
				t.Errorf("Filter() = %v, want %v", gotNames, tt.want)
			}
		})
	}

	t.Run("does not mutate source slice", func(t *testing.T) {
		src := []Repository{{Name: "clean-1h", Dirty: false, LastCommitAt: t1h}, {Name: "dirty-2d", Dirty: true, LastCommitAt: t2d}}
		srcCopy := make([]Repository, len(src))
		copy(srcCopy, src)
		_ = Filter(src, Options{ShowClean: true, ShowDirty: false})
		if !reflect.DeepEqual(src, srcCopy) {
			t.Errorf("Filter mutated source slice: got %v, want %v", src, srcCopy)
		}
	})
}
