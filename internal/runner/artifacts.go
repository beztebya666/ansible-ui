package runner

import (
	"encoding/base64"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

const (
	maxArtifactBytesWS  = 12 << 20  // per-file cap for a single WS frame
	maxArtifactsTotalWS = 100 << 20 // per-run total cap
	maxArtifactsCountWS = 100
)

// sendArtifacts globs the working directory for the spec's artifact paths and
// streams each matching file back to the api as a FrameArtifact. Running on the
// runner means artifacts are captured wherever the run executed — including
// remote runners with no shared /data. Best-effort: errors never fail the run.
func sendArtifacts(conn *safeConn, spec model.ExecSpec, log *slog.Logger) {
	if len(spec.ArtifactPaths) == 0 || spec.Dir == "" {
		return
	}
	var total int64
	count := 0
	seen := map[string]bool{}
	for _, glob := range spec.ArtifactPaths {
		g := strings.TrimSpace(glob)
		if g == "" || strings.Contains(g, "..") { // no path escapes
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(spec.Dir, filepath.FromSlash(g)))
		for _, m := range matches {
			if count >= maxArtifactsCountWS || total >= maxArtifactsTotalWS {
				break
			}
			info, err := os.Stat(m)
			if err != nil || info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(spec.Dir, m)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue // outside the working dir
			}
			name := filepath.ToSlash(rel)
			if seen[name] || strings.HasPrefix(path.Base(name), ".aui-inv-") {
				continue // dedupe; never ship our transient inventory files
			}
			if info.Size() > maxArtifactBytesWS {
				log.Warn("artifact too large to upload, skipped", "run", spec.RunID, "file", name, "size", info.Size())
				continue
			}
			data, rerr := os.ReadFile(m)
			if rerr != nil {
				continue
			}
			if err := conn.send(model.Frame{
				Type: model.FrameArtifact,
				Name: name,
				Data: base64.StdEncoding.EncodeToString(data),
			}); err != nil {
				return // socket gone
			}
			seen[name] = true
			total += info.Size()
			count++
		}
	}
	if count > 0 {
		log.Info("sent artifacts", "run", spec.RunID, "count", count)
	}
}
