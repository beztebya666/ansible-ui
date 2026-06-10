package runner

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nikiv/ansible-ui/internal/model"
)

// Server is the runner's HTTP surface: a health probe and a streaming exec
// endpoint the api connects to over WebSocket.
type Server struct {
	log      *slog.Logger
	upgrader websocket.Upgrader
}

// NewServer builds a runner HTTP handler set.
func NewServer(log *slog.Logger) *Server {
	return &Server{
		log: log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 32 * 1024,
			// Trusted internal service-to-service traffic.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// Handler returns the mux for the runner service.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"runner"}`))
	})
	mux.HandleFunc("GET /v1/exec", s.handleExec)
	mux.HandleFunc("POST /v1/git/sync", s.handleGitSync)
	mux.HandleFunc("POST /v1/git/commit", s.handleGitCommit)
	mux.HandleFunc("POST /v1/inventory/list", s.handleInventoryList)
	mux.HandleFunc("POST /v1/facts/gather", s.handleFactsGather)
	mux.HandleFunc("POST /v1/host/ping", s.handleHostPing)
	mux.HandleFunc("GET /v1/ansible/version", s.handleAnsibleVersion)
	return mux
}

// handleGitCommit writes files, commits them onto a new branch and pushes it.
func (s *Server) handleGitCommit(w http.ResponseWriter, r *http.Request) {
	var spec model.GitCommitSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	s.log.Info("git commit", "repo", spec.RepoID, "branch", spec.NewBranch, "files", len(spec.Files))
	res := Commit(spec)
	if res.Error != "" {
		s.log.Warn("git commit failed", "repo", spec.RepoID, "err", res.Error)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// handleInventoryList resolves an inventory's hosts/groups via ansible-inventory.
func (s *Server) handleInventoryList(w http.ResponseWriter, r *http.Request) {
	var spec model.InventoryListSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	res := ListInventory(spec)
	if res.Error != "" {
		s.log.Warn("inventory list failed", "dir", spec.Dir, "err", res.Error)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// handleFactsGather gathers Ansible facts for an inventory's hosts.
func (s *Server) handleFactsGather(w http.ResponseWriter, r *http.Request) {
	var spec model.FactsGatherSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	res := GatherFacts(spec)
	if res.Error != "" {
		s.log.Warn("facts gather failed", "dir", spec.Dir, "err", res.Error)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// handleAnsibleVersion returns the runner's `ansible --version` output (the
// authoritative core/python/jinja/collection versions, since ansible lives here).
func (s *Server) handleAnsibleVersion(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("ansible", "--version").CombinedOutput()
	w.Header().Set("Content-Type", "application/json")
	if err != nil && len(out) == 0 {
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "", "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"version": string(out)})
}

// handleHostPing checks reachability of an inventory's hosts (ansible -m ping).
func (s *Server) handleHostPing(w http.ResponseWriter, r *http.Request) {
	var spec model.PingSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	res := PingHosts(spec)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// handleGitSync clones/updates a repository and returns the resulting commit.
func (s *Server) handleGitSync(w http.ResponseWriter, r *http.Request) {
	var spec model.GitSyncSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	s.log.Info("git sync", "repo", spec.RepoID, "url", spec.GitURL, "branch", spec.Branch)
	res := Sync(spec)
	if res.Error != "" {
		s.log.Warn("git sync failed", "repo", spec.RepoID, "err", res.Error)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// safeConn serialises writes; gorilla permits one concurrent writer only.
type safeConn struct {
	*websocket.Conn
	mu sync.Mutex
}

func (c *safeConn) send(f model.Frame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.WriteJSON(f)
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	raw, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("ws upgrade failed", "err", err)
		return
	}
	conn := &safeConn{Conn: raw}
	defer conn.Close()
	conn.SetReadLimit(1 << 20)

	// First message is the ExecSpec.
	_, specBytes, err := conn.ReadMessage()
	if err != nil {
		s.log.Warn("read spec failed", "err", err)
		return
	}
	var spec model.ExecSpec
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		_ = conn.send(model.Frame{Type: model.FrameError, Data: "invalid exec spec: " + err.Error()})
		return
	}

	// Install Galaxy requirements (roles/collections) before the play.
	if spec.GalaxyInstall {
		for _, rf := range GalaxyRequirements(spec.Dir) {
			_ = conn.send(stdoutFrame("\r\n\x1b[1;36m▸ ansible-galaxy install -r " + rf + "\x1b[0m\r\n"))
			gp, gerr := StartGalaxy(rf, spec)
			if gerr != nil {
				_ = conn.send(model.Frame{Type: model.FrameError, Data: "galaxy: " + gerr.Error()})
				continue
			}
			if code := streamToCompletion(conn, gp); code != 0 {
				_ = conn.send(stdoutFrame("\r\n\x1b[31m✗ ansible-galaxy failed (exit " + strconv.Itoa(code) + ")\x1b[0m\r\n"))
				_ = conn.send(model.Frame{Type: model.FrameExit, Code: code})
				return
			}
		}
	}

	proc, err := Start(spec)
	if err != nil {
		s.log.Error("start failed", "run", spec.RunID, "err", err)
		_ = conn.send(model.Frame{Type: model.FrameError, Data: err.Error()})
		_ = conn.send(model.Frame{Type: model.FrameExit, Code: -1})
		return
	}
	s.log.Info("exec started", "run", spec.RunID, "pid", proc.PID(), "playbook", spec.Playbook)
	_ = conn.send(model.Frame{Type: model.FrameStarted, PID: proc.PID()})

	// Max task duration: cancel the process when it overruns (0 = unlimited).
	if spec.TimeoutSec > 0 {
		timer := time.AfterFunc(time.Duration(spec.TimeoutSec)*time.Second, func() {
			s.log.Warn("task exceeded max duration — canceling", "run", spec.RunID, "timeoutSec", spec.TimeoutSec)
			msg := fmt.Sprintf("\r\n\x1b[31m✗ task exceeded max duration (%ds) — canceling\x1b[0m\r\n", spec.TimeoutSec)
			_ = conn.send(model.Frame{Type: model.FrameStdout, Data: base64.StdEncoding.EncodeToString([]byte(msg))})
			proc.Cancel()
		})
		defer timer.Stop()
	}

	// Control reader: resize / cancel from the api (relayed from the browser).
	go func() {
		for {
			var f model.Frame
			if err := conn.ReadJSON(&f); err != nil {
				proc.Cancel() // client/api went away — stop the run
				return
			}
			switch f.Type {
			case model.FrameResize:
				_ = proc.Resize(f.Cols, f.Rows)
			case model.FrameCancel:
				s.log.Info("cancel requested", "run", spec.RunID)
				proc.Cancel()
			}
		}
	}()

	// Stream PTY output until the process exits.
	buf := make([]byte, 32*1024)
	for {
		n, rerr := proc.Read(buf)
		if n > 0 {
			if serr := conn.send(model.Frame{
				Type: model.FrameStdout,
				Data: base64.StdEncoding.EncodeToString(buf[:n]),
			}); serr != nil {
				proc.Cancel()
				break
			}
		}
		if rerr != nil {
			if rerr != io.EOF {
				s.log.Debug("pty read ended", "run", spec.RunID, "err", rerr)
			}
			break
		}
	}

	code := proc.Wait()
	s.log.Info("exec finished", "run", spec.RunID, "code", code)
	// Capture + stream artifacts back before signalling exit, so the api has them
	// before it finalises the run (works for remote runners with no shared /data).
	sendArtifacts(conn, spec, s.log)
	_ = conn.send(model.Frame{Type: model.FrameExit, Code: code})
}

// stdoutFrame wraps a string as a stdout frame.
func stdoutFrame(s string) model.Frame {
	return model.Frame{Type: model.FrameStdout, Data: base64.StdEncoding.EncodeToString([]byte(s))}
}

// streamToCompletion pumps a process's PTY output to the socket and returns its
// exit code (used for pre-steps like ansible-galaxy that need no control input).
func streamToCompletion(conn *safeConn, p *Process) int {
	buf := make([]byte, 32*1024)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			if serr := conn.send(model.Frame{
				Type: model.FrameStdout,
				Data: base64.StdEncoding.EncodeToString(buf[:n]),
			}); serr != nil {
				p.Cancel()
				break
			}
		}
		if err != nil {
			break
		}
	}
	return p.Wait()
}
