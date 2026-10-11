package compat

import (
	"errors"
	"testing"

	"github.com/leocov-dev/packwiz-nxt/jarmeta"
)

func TestFabricMatcher(t *testing.T) {
	tests := []struct {
		name    string
		rng     string
		version string
		want    bool
		wantErr error
	}{
		{"any star", "*", "1.20.1", true, nil},
		{"any star snapshot", "*", "24w14a", true, nil},
		{"empty is any", "", "1.20.1", true, nil},
		{"exact match", "1.20.1", "1.20.1", true, nil},
		{"exact mismatch", "1.20.1", "1.20.2", false, nil},
		{"exact short pads zero", "1.20", "1.20.0", true, nil},
		{"exact short vs patch", "1.20", "1.20.1", false, nil},
		{"range both bounds in", ">=1.20 <1.21", "1.20.4", true, nil},
		{"range lower edge", ">=1.20 <1.21", "1.20", true, nil},
		{"range upper edge excluded", ">=1.20 <1.21", "1.21", false, nil},
		{"range below", ">=1.20 <1.21", "1.19.4", false, nil},
		{"upper prerelease below release", ">=1.20 <1.21", "1.21-pre1", true, nil},
		{"prerelease below lower", ">=1.21", "1.21-rc1", false, nil},
		{"prerelease above lower", ">=1.20.5", "1.21-pre1", true, nil},
		{"operator with space", ">= 1.20 < 1.21", "1.20.1", true, nil},
		{"greater strict", ">1.20", "1.20.0", false, nil},
		{"less equal", "<=1.20.1", "1.20.1", true, nil},
		{"equals op", "=1.20.1", "1.20.1", true, nil},
		{"caret in", "^1.20", "1.99.0", true, nil},
		{"caret out major", "^1.20", "2.0.0", false, nil},
		{"caret below", "^1.20", "1.19.9", false, nil},
		{"tilde patch in", "~1.20.1", "1.20.4", true, nil},
		{"tilde patch below", "~1.20.1", "1.20.0", false, nil},
		{"tilde patch out", "~1.20.1", "1.21.0", false, nil},
		{"tilde minor in", "~1.20", "1.20.6", true, nil},
		{"tilde minor out", "~1.20", "1.21", false, nil},
		{"tilde major only", "~1", "1.9.0", true, nil},
		{"wildcard x", "1.20.x", "1.20.4", true, nil},
		{"wildcard x out", "1.20.x", "1.21.0", false, nil},
		{"wildcard star", "1.20.*", "1.20.0", true, nil},
		{"wildcard major", "1.x", "1.5.2", true, nil},
		{"wildcard major out", "1.x", "2.0.0", false, nil},
		{"or alternative second", ">=2.0 || 1.20.x", "1.20.1", true, nil},
		{"open prerelease bound", ">=1.20-", "1.20-pre1", true, nil},
		{"closed bound excludes prerelease", ">=1.20", "1.20-pre1", false, nil},
		{"build metadata ignored", ">=0.92.1", "0.92.1+1.20.1", true, nil},
		{"snapshot exact", "24w14a", "24w14a", true, nil},
		{"snapshot exact mismatch", "24w14a", "24w13a", false, nil},
		{"snapshot vs semver exact", "1.20.1", "24w14a", false, nil},
		{"snapshot vs comparator", ">=1.20", "24w14a", false, ErrInvalidVersion},
		{"snapshot vs wildcard", "1.20.x", "24w14a", false, ErrInvalidVersion},
		{"empty version", ">=1.0", "", false, ErrInvalidVersion},
		{"comparator garbage bound", ">=abc", "1.0.0", false, ErrInvalidRange},
		{"bare operator", ">=", "1.0.0", false, ErrInvalidRange},
		{"bad wildcard", "1.x.3", "1.0.0", false, ErrInvalidRange},
		{"failing term wins over bad term", "<1.0 >=zzz", "2.0.0", false, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FabricMatcher{}.Satisfies(tc.rng, tc.version)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("Satisfies(%q, %q) = %v, want %v", tc.rng, tc.version, got, tc.want)
			}
		})
	}
}

func TestMavenMatcher(t *testing.T) {
	tests := []struct {
		name    string
		rng     string
		version string
		want    bool
		wantErr error
	}{
		{"empty any", "", "1.20.1", true, nil},
		{"star any", "*", "1.20.1", true, nil},
		{"bare soft", "1.0", "0.1", true, nil},
		{"half open in", "[1.20,1.21)", "1.20.1", true, nil},
		{"half open lower edge", "[1.20,1.21)", "1.20", true, nil},
		{"half open upper edge", "[1.20,1.21)", "1.21", false, nil},
		{"half open below", "[1.20,1.21)", "1.19.4", false, nil},
		{"exclusive lower", "(1.0,2.0]", "1.0", false, nil},
		{"inclusive upper", "(1.0,2.0]", "2.0", true, nil},
		{"exact in", "[1.0]", "1.0.0", true, nil},
		{"exact out", "[1.0]", "1.0.1", false, nil},
		{"open upper", "[1.0,)", "99.0", true, nil},
		{"open upper below", "[1.0,)", "0.9", false, nil},
		{"open lower", "(,1.0]", "0.5", true, nil},
		{"open lower above", "(,1.0]", "1.0.1", false, nil},
		{"union first", "[1.0,2.0),[3.0,)", "1.5", true, nil},
		{"union gap", "[1.0,2.0),[3.0,)", "2.5", false, nil},
		{"union second", "[1.0,2.0),[3.0,)", "3.1", true, nil},
		{"union spaces", "[1.0,2.0), [3.0,)", "3.1", true, nil},
		{"numeric not lexical", "[1.9,1.10]", "1.10", true, nil},
		{"numeric not lexical above", "[1.9,1.10)", "1.10", false, nil},
		{"neoforge version", "[21.1,)", "21.1.172", true, nil},
		{"neoforge below", "[21.2,)", "21.1.172", false, nil},
		{"forge major only", "[47,)", "47.2.0", true, nil},
		{"mc range", "[1.21,1.22)", "1.21.1", true, nil},
		{"beta below release", "[1.0,)", "1.0-beta", false, nil},
		{"beta in range below", "(,1.0)", "1.0-beta", true, nil},
		{"rc between", "[1.0-beta,1.0)", "1.0-rc1", true, nil},
		{"release above rc", "(,1.0-rc1]", "1.0", false, nil},
		{"alpha digit transition", "[1.0,)", "1.0.1-alpha2", true, nil},
		{"garbage range", "[1.0,2.0", "1.5", false, ErrInvalidRange},
		{"garbage between", "[1.0,2.0) x", "1.5", false, ErrInvalidRange},
		{"trailing comma", "[1.0,2.0),", "1.5", false, ErrInvalidRange},
		{"single needs brackets", "(1.0)", "1.0", false, ErrInvalidRange},
		{"too many bounds", "[1,2,3]", "1.5", false, ErrInvalidRange},
		{"bad bound", "[1.0,@)", "1.5", false, ErrInvalidRange},
		{"bare with bracket", "1.0]", "1.0", false, ErrInvalidRange},
		{"garbage version", "[1.0,)", "!!", false, ErrInvalidVersion},
		{"empty version", "[1.0,)", "", false, ErrInvalidVersion},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MavenMatcher{}.Satisfies(tc.rng, tc.version)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("Satisfies(%q, %q) = %v, want %v", tc.rng, tc.version, got, tc.want)
			}
		})
	}
}

func TestSatisfiesDispatch(t *testing.T) {
	tests := []struct {
		loader  jarmeta.Loader
		rng     string
		version string
		want    bool
		wantErr error
	}{
		{jarmeta.LoaderFabric, ">=1.20 <1.21", "1.20.1", true, nil},
		{jarmeta.LoaderQuilt, "^1.20", "1.20.1", true, nil},
		{jarmeta.LoaderForge, "[1.20,1.21)", "1.20.1", true, nil},
		{jarmeta.LoaderNeoForge, "[21,)", "21.1.1", true, nil},
		{jarmeta.Loader("liteloader"), "*", "1", false, ErrUnsupportedLoader},
	}
	for _, tc := range tests {
		got, err := Satisfies(tc.loader, tc.rng, tc.version)
		if !errors.Is(err, tc.wantErr) || got != tc.want {
			t.Errorf("Satisfies(%s,%q,%q) = %v, %v; want %v, %v", tc.loader, tc.rng, tc.version, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestEvalDependencyRanges(t *testing.T) {
	m := FabricMatcher{}
	tests := []struct {
		name string
		dep  jarmeta.Dependency
		ver  string
		want verdict
	}{
		{"no ranges", jarmeta.Dependency{}, "1.0.0", verdictYes},
		{"any-of one matches", jarmeta.Dependency{VersionRanges: []string{"1.x", "3.x"}}, "3.1.0", verdictYes},
		{"any-of none", jarmeta.Dependency{VersionRanges: []string{"1.x", "3.x"}}, "2.0.0", verdictNo},
		{"all-of fails", jarmeta.Dependency{VersionRangesAll: []string{">=1", "<2"}}, "2.0.0", verdictNo},
		{"all-of passes", jarmeta.Dependency{VersionRangesAll: []string{">=1", "<2"}}, "1.5.0", verdictYes},
		{"any-of bad range unknown", jarmeta.Dependency{VersionRanges: []string{">=zz"}}, "1.0.0", verdictUnknown},
		{"any-of match beats bad range", jarmeta.Dependency{VersionRanges: []string{">=zz", "1.x"}}, "1.0.0", verdictYes},
		{"no beats unknown", jarmeta.Dependency{VersionRanges: []string{"3.x"}, VersionRangesAll: []string{">=zz"}}, "1.0.0", verdictNo},
		{"unknown any with all yes", jarmeta.Dependency{VersionRanges: []string{">=zz"}, VersionRangesAll: []string{">=1"}}, "1.0.0", verdictUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := evalDependencyRanges(m, tc.dep, tc.ver)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
