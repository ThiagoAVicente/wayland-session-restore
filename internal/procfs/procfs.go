// Package procfs reads process info (cmdline, cwd) from /proc.
package procfs

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Root is the default proc filesystem mount point.
const Root = "/proc"

// Cmdline returns the argv of pid, or nil if unreadable.
func Cmdline(procRoot string, pid int) []string {
	raw, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return nil
	}
	var out []string
	for _, p := range bytes.Split(raw, []byte{0}) {
		if len(p) > 0 {
			out = append(out, string(p))
		}
	}
	return out
}

func cwd(procRoot string, pid int) string {
	p, err := filepath.EvalSymlinks(filepath.Join(procRoot, strconv.Itoa(pid), "cwd"))
	if err != nil {
		return ""
	}
	return p
}

// ppidMap returns pid -> ppid for all readable processes.
func ppidMap(procRoot string) map[int]int {
	out := map[int]int{}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return out
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join(procRoot, e.Name(), "stat"))
		if err != nil {
			continue
		}
		// ppid is field 4; comm (field 2) may contain spaces, so split after last ')'
		s := string(stat)
		i := strings.LastIndexByte(s, ')')
		if i < 0 {
			continue
		}
		fields := strings.Fields(s[i+1:])
		if len(fields) < 2 {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		out[pid] = ppid
	}
	return out
}

// DeepestCwd returns the cwd of the deepest descendant of pid, walking the
// newest child at each level. Terminals keep their launch cwd; the shell or
// editor underneath holds the cwd worth restoring. Empty string if unknown.
func DeepestCwd(procRoot string, pid int) string {
	if _, err := os.Stat(filepath.Join(procRoot, strconv.Itoa(pid))); err != nil {
		return ""
	}
	children := map[int][]int{}
	for p, pp := range ppidMap(procRoot) {
		children[pp] = append(children[pp], p)
	}
	current, result := pid, cwd(procRoot, pid)
	for len(children[current]) > 0 {
		next := children[current][0]
		for _, c := range children[current] {
			if c > next {
				next = c // newest child on ties
			}
		}
		current = next
		if c := cwd(procRoot, current); c != "" {
			result = c
		}
	}
	return result
}
