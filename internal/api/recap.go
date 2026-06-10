package api

import (
	"regexp"
	"strconv"

	"github.com/nikiv/ansible-ui/internal/model"
)

var (
	ansiRe  = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")
	recapRe = regexp.MustCompile(`ok=(\d+)\s+changed=(\d+)\s+unreachable=(\d+)\s+failed=(\d+)\s+skipped=(\d+)\s+rescued=(\d+)\s+ignored=(\d+)`)
)

// parseRecap extracts aggregate counts from a playbook's PLAY RECAP, summing
// across hosts. Output may contain ANSI colour, which is stripped first.
func parseRecap(out []byte) model.RunStats {
	clean := ansiRe.ReplaceAll(out, nil)
	var st model.RunStats
	for _, m := range recapRe.FindAllSubmatch(clean, -1) {
		st.Hosts++
		st.Ok += atoi(m[1])
		st.Changed += atoi(m[2])
		st.Unreachable += atoi(m[3])
		st.Failed += atoi(m[4])
		st.Skipped += atoi(m[5])
		st.Rescued += atoi(m[6])
		st.Ignored += atoi(m[7])
	}
	return st
}

func atoi(b []byte) int {
	n, _ := strconv.Atoi(string(b))
	return n
}
