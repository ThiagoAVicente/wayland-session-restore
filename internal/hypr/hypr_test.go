package hypr

import (
	"strings"
	"testing"
)

func TestClientsFiltersUnmappedPidlessSpecial(t *testing.T) {
	fake := `[
	 {"class":"foot","pid":123,"mapped":true,"workspace":{"id":1}},
	 {"class":"bad","pid":-1,"mapped":true,"workspace":{"id":1}},
	 {"class":"hidden","pid":99,"mapped":false,"workspace":{"id":1}},
	 {"class":"scratch","pid":50,"mapped":true,"workspace":{"id":-98}}
	]`
	var gotArgs []string
	run = func(args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(fake), nil
	}
	t.Cleanup(func() { run = defaultRun })

	cs, err := Clients()
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Class != "foot" {
		t.Fatalf("got %+v", cs)
	}
	if strings.Join(gotArgs, " ") != "-j clients" {
		t.Fatalf("hyprctl args: %v", gotArgs)
	}
}

func TestDispatchExecBuildsRulePrefix(t *testing.T) {
	var gotArgs []string
	run = func(args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}
	t.Cleanup(func() { run = defaultRun })

	if err := DispatchExec([]string{"foot", "-e", "htop"}, "workspace 2 silent"); err != nil {
		t.Fatal(err)
	}
	want := []string{"dispatch", "exec", "[workspace 2 silent] foot -e htop"}
	if len(gotArgs) != 3 || gotArgs[0] != want[0] || gotArgs[1] != want[1] || gotArgs[2] != want[2] {
		t.Fatalf("got %q", gotArgs)
	}
}

func TestDispatchExecQuotesArgsWithSpaces(t *testing.T) {
	var got string
	run = func(args ...string) ([]byte, error) {
		got = args[len(args)-1]
		return nil, nil
	}
	t.Cleanup(func() { run = defaultRun })

	DispatchExec([]string{"sh", "-c", "cd /tmp && exec foot"}, "")
	if got != `sh -c 'cd /tmp && exec foot'` {
		t.Fatalf("got %q", got)
	}
}

func TestShQuoteSingleQuotes(t *testing.T) {
	if got := shQuote("it's"); got != `'it'\''s'` {
		t.Fatalf("got %q", got)
	}
}
