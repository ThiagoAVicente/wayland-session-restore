package procfs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func makeProc(t *testing.T, root string, pid, ppid int, cmd []string, cwd string) {
	t.Helper()
	d := filepath.Join(root, fmt.Sprint(pid))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	for _, c := range cmd {
		raw = append(raw, c...)
		raw = append(raw, 0)
	}
	os.WriteFile(filepath.Join(d, "cmdline"), raw, 0o644)
	os.WriteFile(filepath.Join(d, "stat"),
		[]byte(fmt.Sprintf("%d (%s) S %d 0 0", pid, cmd[0], ppid)), 0o644)
	os.MkdirAll(cwd, 0o755)
	os.Symlink(cwd, filepath.Join(d, "cwd"))
}

func TestCmdline(t *testing.T) {
	root := t.TempDir()
	makeProc(t, root, 100, 1, []string{"foot", "--app-id=x"}, filepath.Join(root, "home"))
	got := Cmdline(root, 100)
	if len(got) != 2 || got[0] != "foot" || got[1] != "--app-id=x" {
		t.Fatalf("got %v", got)
	}
}

func TestCmdlineMissingPid(t *testing.T) {
	if got := Cmdline(t.TempDir(), 999); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestDeepestCwdFollowsChildChain(t *testing.T) {
	root := t.TempDir()
	// foot(100) -> zsh(200) -> nvim(300); nvim's cwd wins
	makeProc(t, root, 100, 1, []string{"foot"}, filepath.Join(root, "a"))
	makeProc(t, root, 200, 100, []string{"zsh"}, filepath.Join(root, "b"))
	makeProc(t, root, 300, 200, []string{"nvim"}, filepath.Join(root, "b", "proj"))
	want, _ := filepath.EvalSymlinks(filepath.Join(root, "b", "proj"))
	if got := DeepestCwd(root, 100); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDeepestCwdNoChildrenReturnsOwn(t *testing.T) {
	root := t.TempDir()
	makeProc(t, root, 100, 1, []string{"firefox"}, filepath.Join(root, "home"))
	want, _ := filepath.EvalSymlinks(filepath.Join(root, "home"))
	if got := DeepestCwd(root, 100); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDeepestCwdUnreadableReturnsEmpty(t *testing.T) {
	if got := DeepestCwd(t.TempDir(), 999); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
