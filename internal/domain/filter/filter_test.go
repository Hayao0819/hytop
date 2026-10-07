package filter_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

// tree is
//
//	1  systemd      root
//	├── 10 sshd     root
//	│   ├── 11 bash hayao   cpu 0.3
//	│   └── 12 bash hayao   cpu 8.0
//	│       └── 13 vim      hayao  cpu 0.5  rss 2G
//	└── 20 firefox  hayao   cpu 40   gpu 30
//	2  kthreadd     root    kthread
//	└── 3  kworker  root    kthread
//	40 dockerd      root    unit docker.service
//	41 nginx        root    container abc123  port 80,443
func tree() *procmodel.Snapshot {
	return procmodel.NewSnapshot([]procmodel.Process{
		{PID: 1, PPID: 0, Name: "systemd", User: "root", CPU: 0.1, RSS: 10 << 20},
		{PID: 10, PPID: 1, Name: "sshd", User: "root", CPU: 0.2, RSS: 20 << 20},
		{PID: 11, PPID: 10, Name: "bash", User: "hayao", CPU: 0.3, RSS: 30 << 20},
		{PID: 12, PPID: 10, Name: "bash", User: "hayao", CPU: 8, RSS: 40 << 20},
		{PID: 13, PPID: 12, Name: "vim", User: "hayao", UID: 1000, CPU: 0.5, RSS: 2 << 30},
		{PID: 20, PPID: 1, Name: "firefox", User: "hayao", CPU: 40, RSS: 900 << 20, GPUPc: 30},
		{PID: 2, PPID: 0, Name: "kthreadd", User: "root", Kthread: true},
		{PID: 3, PPID: 2, Name: "kworker/0:1", User: "root", Kthread: true},
		{PID: 40, PPID: 1, Name: "dockerd", User: "root", Unit: "docker.service"},
		{PID: 41, PPID: 40, Name: "nginx", User: "root", Container: "abc123", Ports: []int{80, 443}},
	})
}

func selectPIDs(t *testing.T, src string, vars filter.Vars) []int {
	t.Helper()

	expr, err := filter.Parse(src, vars)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}

	pids, err := filter.Select(expr, tree())
	if err != nil {
		t.Fatal(err)
	}

	return pids
}

func TestComparisons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src  string
		want []int
	}{
		{`pid == 13`, []int{13}},
		{`name == "bash"`, []int{11, 12}},
		{`name == bash`, []int{11, 12}},
		{`name ~ "^k"`, []int{2, 3}},
		{`unit ^= "docker-"`, nil},
		{`unit ^= "docker"`, []int{40}},
		{`user == "hayao" and cpu > 1`, []int{12, 20}},
		{`cpu > 1 or gpu.util > 0`, []int{12, 20}},
		{`rss > 1G`, []int{13}},
		{`rss >= 900M`, []int{13, 20}},
		{`container != ""`, []int{41}},
		{`port == 443`, []int{41}},
		{`port == 8080`, nil},
		{`kthread`, []int{2, 3}},
		{`threads == 0 and not kthread`, []int{1, 10, 11, 12, 13, 20, 40, 41}},
	}

	for _, tt := range tests {
		got := selectPIDs(t, tt.src, nil)
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.src, got, tt.want)
		}
	}
}

func TestStructuralRelations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src  string
		want []int
	}{
		{`children(name == "sshd")`, []int{11, 12}},
		{`descendants(name == "sshd")`, []int{11, 12, 13}},
		{`subtree(name == "sshd")`, []int{10, 11, 12, 13}},
		{`subtree(pid == 12)`, []int{12, 13}},
		{`ancestors(name == "vim")`, []int{1, 10, 12}},
		{`siblings(pid == 11)`, []int{12}},
		{`descendants(name ~ "^firefox")`, nil},
		{`subtree(pid == 10) and cpu > 1`, []int{12}},
		{`not subtree(pid == 2)`, []int{1, 10, 11, 12, 13, 20, 40, 41}},
	}

	for _, tt := range tests {
		got := selectPIDs(t, tt.src, nil)
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.src, got, tt.want)
		}
	}
}

func TestVariables(t *testing.T) {
	t.Parallel()

	vars := filter.Vars{"USER": "hayao", "PPID": "10"}

	if got := selectPIDs(t, `user == $USER and name == "bash"`, vars); !slices.Equal(got, []int{11, 12}) {
		t.Errorf("$USER = %v", got)
	}

	if got := selectPIDs(t, `subtree(pid == $PPID)`, vars); !slices.Equal(got, []int{10, 11, 12, 13}) {
		t.Errorf("$PPID = %v", got)
	}

	if _, err := filter.Parse(`user == $NOPE`, vars); err == nil {
		t.Error("an unset variable parsed without error")
	}
}

func TestPrecedenceAndGrouping(t *testing.T) {
	t.Parallel()

	loose := selectPIDs(t, `name == "vim" or name == "bash" and cpu > 1`, nil)
	if !slices.Equal(loose, []int{12, 13}) {
		t.Errorf("without parens = %v", loose)
	}

	tight := selectPIDs(t, `(name == "vim" or name == "bash") and cpu > 1`, nil)
	if !slices.Equal(tight, []int{12}) {
		t.Errorf("with parens = %v", tight)
	}

	if got := selectPIDs(t, `not not kthread`, nil); !slices.Equal(got, []int{2, 3}) {
		t.Errorf("double negation = %v", got)
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{
		``,
		`name ==`,
		`name == "unterminated`,
		`nosuchfield == 1`,
		`cpu`,
		`name > 3`,
		`cpu ~ "x"`,
		`subtree(`,
		`(name == "a"`,
		`name == "a" extra`,
		`name ~ "("`,
		`rss > 1Q`,
	} {
		if _, err := filter.Parse(bad, nil); err == nil {
			t.Errorf("Parse(%q) = nil error, want one", bad)
		}
	}
}

func TestStringRoundTripsThroughTheParser(t *testing.T) {
	t.Parallel()

	expr, err := filter.Parse(`descendants(name ~ "^firefox") and cpu > 1`, nil)
	if err != nil {
		t.Fatal(err)
	}

	again, err := filter.Parse(expr.String(), nil)
	if err != nil {
		t.Fatalf("reparsing %q: %v", expr.String(), err)
	}

	if again.String() != expr.String() {
		t.Errorf("round trip: %q -> %q", expr.String(), again.String())
	}
}

func TestNilExprSelectsEverything(t *testing.T) {
	t.Parallel()

	if got, err := filter.Select(nil, tree()); err != nil || len(got) != 10 {
		t.Errorf("Select(nil) = %d processes, want all 10", len(got))
	}
}

func TestFieldsAreListedForHelp(t *testing.T) {
	t.Parallel()

	names := filter.Fields()

	if !slices.IsSorted(names) {
		t.Error("Fields is not sorted")
	}

	if !slices.Contains(names, "subtree") && !slices.Contains(names, "cpu") {
		t.Errorf("Fields = %v", names)
	}
}

func TestExprPredicateAndRelation(t *testing.T) {
	t.Parallel()

	expr, err := filter.Compile(`(cpu >= 8 && name in ["bash", "firefox"]) || uid == 1000`)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := filter.Select(expr, tree()); err != nil || !slices.Equal(got, []int{12, 13, 20}) {
		t.Fatalf("direct Expr selected %v", got)
	}

	got := selectPIDs(t, `subtree(expr("name startsWith \"fire\" && cpu >= 10"))`, nil)
	if !slices.Equal(got, []int{20}) {
		t.Fatalf("Expr relation selected %v", got)
	}

	if _, err := filter.Compile(`expr("unknown > 0")`); err == nil {
		t.Fatal("unknown Expr field accepted")
	}
}

func TestExprNestedFieldsCollectionsAndRoundTrip(t *testing.T) {
	t.Parallel()

	compiled, err := filter.Compile(`gpu.util > 0 || 443 in port || io.read > 0`)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := filter.Select(compiled, tree()); err != nil || !slices.Equal(got, []int{20, 41}) {
		t.Fatalf("nested Expr selected %v", got)
	}

	source := compiled.String()
	again, err := filter.Compile(source)
	if err != nil {
		t.Fatalf("recompile %q: %v", source, err)
	}
	if got, err := filter.Select(again, tree()); err != nil || !slices.Equal(got, []int{20, 41}) {
		t.Fatalf("round-tripped Expr selected %v", got)
	}
}

func TestExprEvaluationErrorsAreReported(t *testing.T) {
	t.Parallel()

	expr, err := filter.Compile(`port[100] > 0`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := filter.Select(expr, tree()); err == nil {
		t.Fatal("out-of-range access was treated as an empty match")
	}
}

func TestCompileKeepsBothParserDiagnostics(t *testing.T) {
	t.Parallel()

	_, err := filter.Compile(`cpu >`)
	if err == nil || !strings.Contains(err.Error(), "Expr") || !strings.Contains(err.Error(), "filter:") {
		t.Fatalf("Compile error = %v", err)
	}
}
