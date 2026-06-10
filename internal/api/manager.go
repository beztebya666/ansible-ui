package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/notify"
	"github.com/nikiv/ansible-ui/internal/runner"
	"github.com/nikiv/ansible-ui/internal/runnerclient"
	"github.com/nikiv/ansible-ui/internal/store"
)

// Manager orchestrates live runs: it relays the runner's PTY stream to any
// number of browser subscribers, persists the full output for replay, and
// publishes lifecycle events.
type Manager struct {
	store      *store.Store
	runnerURL  string
	log        *slog.Logger
	events     *EventHub
	onFinish   func(runnerID string)                             // notified when a run reaches a terminal state
	onArtifact func(run *model.Run, name string, content []byte) // a runner streamed back a captured file
	onTerminal func(run *model.Run, status string)               // run reached a terminal state (advances workflows)

	mu     sync.Mutex
	active map[string]*activeRun
}

type activeRun struct {
	id        string
	mu        sync.Mutex
	buf       bytes.Buffer
	lineTimes []int64 // wall-clock ms for each completed (\n-terminated) output line
	subs      map[chan []byte]struct{}
	conn      *runnerclient.Conn
	canceled  bool
	finished  bool
}

// NewManager builds a run Manager.
func NewManager(st *store.Store, runnerURL string, events *EventHub, log *slog.Logger) *Manager {
	return &Manager{
		store:     st,
		runnerURL: runnerURL,
		log:       log,
		events:    events,
		active:    make(map[string]*activeRun),
	}
}

// Start launches a run in the background and returns immediately. dialURL is the
// runner to execute on; empty falls back to the configured built-in runner.
func (m *Manager) Start(run *model.Run, spec model.ExecSpec, dialURL string) {
	if dialURL == "" {
		dialURL = m.runnerURL
	}
	ar := &activeRun{id: run.ID, subs: make(map[chan []byte]struct{})}
	m.mu.Lock()
	m.active[run.ID] = ar
	m.mu.Unlock()
	go m.execute(run, spec, ar, dialURL)
}

func (m *Manager) execute(run *model.Run, spec model.ExecSpec, ar *activeRun, dialURL string) {
	ctx := context.Background()
	// The displayed/recorded command uses run.ExtraVars (secrets masked), never the
	// spec's real values — so the log + run.args never carry a raw secret.
	displaySpec := spec
	displaySpec.ExtraVars = run.ExtraVars
	args := runner.BuildArgs(displaySpec)

	// A short runner preamble (what's about to happen) so the log opens with
	// context — like Semaphore — instead of cold-starting on ansible's own banner.
	ar.append([]byte(buildPreamble(run, spec, args)))

	conn, err := runnerclient.Dial(ctx, dialURL, spec)
	if err != nil {
		m.log.Error("dial runner failed", "run", run.ID, "err", err)
		ar.append([]byte("\x1b[31m✗ failed to reach runner service: " + err.Error() + "\x1b[0m\r\n"))
		m.finish(run, ar, -1)
		return
	}
	ar.mu.Lock()
	ar.conn = conn
	ar.mu.Unlock()

	// Cross-replica cancel: another replica (where this run isn't streaming) can
	// flag cancel_requested in Postgres; poll it and cancel the live process here.
	cancelDone := make(chan struct{})
	defer close(cancelDone)
	go m.watchCancel(run.ID, cancelDone)

	now := time.Now()
	run.Status = model.StatusRunning
	run.Args = args
	run.StartedAt = &now
	_ = m.store.MarkRunning(ctx, run.ID, args)
	m.publishRun(run)

	gotExit := false
	exitCode := -1
	var lastFlush time.Time
	for {
		f, rerr := conn.Recv()
		if rerr != nil {
			break
		}
		switch f.Type {
		case model.FrameStdout:
			if data, derr := base64.StdEncoding.DecodeString(f.Data); derr == nil {
				ar.append(data)
			}
		case model.FrameError:
			ar.append([]byte("\r\n\x1b[31m[runner] " + f.Data + "\x1b[0m\r\n"))
		case model.FrameArtifact:
			if data, derr := base64.StdEncoding.DecodeString(f.Data); derr == nil && m.onArtifact != nil {
				m.onArtifact(run, f.Name, data)
			}
		case model.FrameExit:
			exitCode = f.Code
			gotExit = true
		}
		// Throttled flush so the live structured log view can show a running run
		// (the xterm streams over WS; this persists for the polled /log endpoint).
		if !gotExit && time.Since(lastFlush) > time.Second {
			ar.mu.Lock()
			out := append([]byte(nil), ar.buf.Bytes()...)
			lt := append([]int64(nil), ar.lineTimes...)
			ar.mu.Unlock()
			_ = m.store.UpdateRunLog(ctx, run.ID, out, lt)
			lastFlush = time.Now()
		}
		if gotExit {
			break
		}
	}
	_ = conn.Close()
	if !gotExit {
		ar.append([]byte("\r\n\x1b[33m[runner] stream ended unexpectedly\x1b[0m\r\n"))
	}
	m.finish(run, ar, exitCode)
}

func (ar *activeRun) append(b []byte) {
	ar.mu.Lock()
	ar.buf.Write(b)
	// Stamp each newly-completed line with the arrival time (≈ when ansible
	// emitted it on a live PTY) so the structured log can show a timestamp gutter.
	if n := bytes.Count(b, []byte{'\n'}); n > 0 {
		now := time.Now().UnixMilli()
		for i := 0; i < n; i++ {
			ar.lineTimes = append(ar.lineTimes, now)
		}
	}
	for ch := range ar.subs {
		select {
		case ch <- b:
		default: // slow consumer — drop; replay from stored output reconciles
		}
	}
	ar.mu.Unlock()
}

func (m *Manager) finish(run *model.Run, ar *activeRun, exitCode int) {
	ctx := context.Background()

	ar.mu.Lock()
	output := append([]byte(nil), ar.buf.Bytes()...)
	lineTimes := append([]int64(nil), ar.lineTimes...)
	canceled := ar.canceled
	ar.finished = true
	for ch := range ar.subs {
		delete(ar.subs, ch)
		close(ch)
	}
	ar.mu.Unlock()

	status := model.StatusSuccess
	switch {
	case canceled:
		status = model.StatusCanceled
	case exitCode != 0:
		status = model.StatusFailed
	}
	stats := parseRecap(output)

	if err := m.store.FinishRun(ctx, run.ID, status, exitCode, stats, output, lineTimes); err != nil {
		m.log.Error("persist run result failed", "run", run.ID, "err", err)
	}

	m.recordTerminal(ctx, run, status)
	// Artifacts are streamed back by the runner as FrameArtifact during the run
	// (handled in the frame loop above), so they're already saved by now.

	fin := time.Now()
	run.Status = status
	run.ExitCode = &exitCode
	run.Stats = stats
	run.FinishedAt = &fin
	m.publishRun(run)

	m.mu.Lock()
	delete(m.active, run.ID)
	m.mu.Unlock()
	// Free the runner slot and drain any queued runs that now fit.
	if m.onFinish != nil {
		m.onFinish(run.RunnerID)
	}
	m.log.Info("run finished", "run", run.ID, "status", status, "code", exitCode)
}

// Subscribe attaches a live consumer to an active run. It returns the output
// accumulated so far plus a channel of subsequent chunks. ok is false if the
// run is not active (the caller should then replay stored output instead).
func (m *Manager) Subscribe(runID string) (snapshot []byte, ch <-chan []byte, unsub func(), ok bool) {
	m.mu.Lock()
	ar := m.active[runID]
	m.mu.Unlock()
	if ar == nil {
		return nil, nil, nil, false
	}
	ar.mu.Lock()
	defer ar.mu.Unlock()
	if ar.finished {
		return nil, nil, nil, false
	}
	snap := append([]byte(nil), ar.buf.Bytes()...)
	c := make(chan []byte, 2048)
	ar.subs[c] = struct{}{}
	return snap, c, func() {
		ar.mu.Lock()
		if _, exists := ar.subs[c]; exists {
			delete(ar.subs, c)
			close(c)
		}
		ar.mu.Unlock()
	}, true
}

// Control forwards a resize/cancel frame to the runner driving the run.
func (m *Manager) Control(runID string, f model.Frame) {
	m.mu.Lock()
	ar := m.active[runID]
	m.mu.Unlock()
	if ar == nil {
		return
	}
	ar.mu.Lock()
	if f.Type == model.FrameCancel {
		ar.canceled = true
	}
	conn := ar.conn
	ar.mu.Unlock()
	if conn != nil {
		_ = conn.Send(f)
	}
}

// watchCancel polls Postgres for a cross-replica cancel request and cancels the
// live run when one appears. Stops when the run loop closes done.
func (m *Manager) watchCancel(runID string, done <-chan struct{}) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			if req, _ := m.store.CancelRequested(context.Background(), runID); req {
				m.Control(runID, model.Frame{Type: model.FrameCancel})
				return
			}
		}
	}
}

// IsActive reports whether a run is currently streaming.
func (m *Manager) IsActive(runID string) bool {
	m.mu.Lock()
	_, ok := m.active[runID]
	m.mu.Unlock()
	return ok
}

func (m *Manager) publishRun(run *model.Run) {
	m.events.Publish(map[string]any{"type": "run.updated", "run": run})
}

// buildPreamble is the short, Semaphore-style header shown before a run's tool
// output, so the log opens with context (what was queued, who launched it, the
// synced commit, the inventory) instead of cold-starting on ansible's own banner.
func buildPreamble(run *model.Run, spec model.ExecSpec, args []string) string {
	name := run.Name
	if name == "" {
		name = run.Playbook
	}
	actor := run.TriggeredBy
	if actor == "" {
		actor = "system"
	}
	app := run.App
	if app == "" {
		app = "ansible"
	}
	var b strings.Builder
	line := func(s string) { b.WriteString(s + "\r\n") }
	line("Task " + name + " added to queue")
	line("Started run #" + strings.TrimPrefix(run.ID, "run_") + " · by " + actor)
	if run.ProjectName != "" {
		line("Project: " + run.ProjectName)
	}
	if run.Playbook != "" {
		line("App: " + app + " · " + run.Playbook)
	} else {
		line("App: " + app)
	}
	line("Preparing workspace…")
	if spec.RepoURL != "" {
		br := spec.RepoBranch
		if br == "" {
			br = "default branch"
		}
		line("Updating repository " + spec.RepoURL + " (branch " + br + ")")
	}
	if run.Commit != "" {
		c := run.Commit
		if len(c) > 10 {
			c = c[:10]
		}
		l := "Checked out " + c
		if spec.CommitMsg != "" {
			l += " — " + spec.CommitMsg
		}
		line(l)
	}
	if spec.Inventory != "" {
		line("Using inventory: " + spec.Inventory)
	}
	if len(args) > 0 {
		bin := ""
		if app == "ansible" {
			bin = "ansible-playbook "
		}
		line("$ " + bin + strings.Join(args, " "))
	}
	line("") // blank line before the tool output
	return b.String()
}

// dispatchNotifications sends run-completion alerts to enabled channels whose
// event filter matches the final status (empty filter = all terminal states).
// recordTerminal logs the run's terminal activity and fires notifications. It is
// shared by finish() (executed runs) and the dispatch helpers (failRun /
// cancelQueued) so a run that never started still audits + alerts consistently.
func (m *Manager) recordTerminal(ctx context.Context, run *model.Run, status string) {
	act := &model.Activity{Actor: run.TriggeredBy, Action: "run." + status, Target: run.Name, Detail: run.App}
	if err := m.store.AddActivity(ctx, act); err == nil {
		m.events.Publish(map[string]any{"type": "activity", "activity": act})
	}
	go m.dispatchNotifications(run, status)
	// Advance any workflow this run belongs to (no-op for ordinary runs).
	if m.onTerminal != nil {
		m.onTerminal(run, status)
	}
}

func (m *Manager) dispatchNotifications(run *model.Run, status string) {
	ctx := context.Background()
	var tpl *model.Template
	if run.TemplateID != nil {
		tpl, _ = m.store.GetTemplate(ctx, *run.TemplateID)
	}
	// A template can mute every notification (Semaphore: "disable all per template").
	if tpl != nil && tpl.SuppressAllNotifications {
		return
	}
	// A "recovery" = success after the template's previous terminal run failed.
	// Channels can subscribe to the synthetic "fixed" event to alert only on these.
	recovered := false
	if status == model.StatusSuccess && run.TemplateID != nil {
		if prev, _ := m.store.PrevTerminalStatus(ctx, *run.TemplateID, run.ID); prev == model.StatusFailed || prev == model.StatusCanceled {
			recovered = true
		}
	}
	// Honour the template's "suppress success notifications" flag — but never
	// suppress a recovery (that's the whole point of fixed-notifications).
	if status == model.StatusSuccess && !recovered && tpl != nil && tpl.SuppressSuccessNotifications {
		return
	}
	chans, err := m.store.ListEnabledNotificationChannels(ctx)
	if err != nil || len(chans) == 0 {
		return
	}
	// The run object from the executor isn't list-hydrated — fill the project name
	// so {{project}} (and the default body) aren't blank.
	if run.ProjectName == "" && run.ProjectID != "" {
		if p, perr := m.store.GetProject(ctx, run.ProjectID); perr == nil {
			run.ProjectName = p.Name
		}
	}
	label := status
	if recovered {
		label = "recovered ✓"
	}
	title := "ansible·ui · " + run.Name + " — " + label
	text := "app=" + run.App + " · project=" + run.ProjectName
	if run.ExitCode != nil {
		text += " · exit=" + strconv.Itoa(*run.ExitCode)
	}
	vars := notifyVars(run, status)
	for _, ch := range chans {
		if ch.ProjectID != nil && *ch.ProjectID != "" && *ch.ProjectID != run.ProjectID {
			continue // channel scoped to a different project
		}
		if !eventMatches(ch.Events, status) && !(recovered && hasEvent(ch.Events, "fixed")) {
			continue
		}
		body := text
		if tmpl := strings.TrimSpace(ch.Template); tmpl != "" {
			body = renderNotifyTemplate(ch.Template, vars)
		}
		evt := status
		if recovered {
			evt = "fixed"
		}
		entry := &model.NotificationLog{
			ChannelID: ch.ID, ChannelName: ch.Name, ChannelType: ch.Type,
			RunID: run.ID, RunName: run.Name, ProjectID: run.ProjectID, Event: evt, OK: true,
		}
		if err := notify.Dispatch(ctx, ch, title, body, "", vars); err != nil {
			m.log.Warn("notification failed", "channel", ch.ID, "type", ch.Type, "err", err)
			entry.OK = false
			entry.Error = err.Error()
		}
		_ = m.store.LogNotification(ctx, entry)
	}
}

// notifyVars are the substitution values available to a channel's message template.
func notifyVars(run *model.Run, status string) map[string]string {
	exit := ""
	if run.ExitCode != nil {
		exit = strconv.Itoa(*run.ExitCode)
	}
	return map[string]string{
		"run":      run.Name,
		"status":   status,
		"app":      run.App,
		"project":  run.ProjectName,
		"playbook": run.Playbook,
		"commit":   run.Commit,
		"version":  run.Version,
		"actor":    run.TriggeredBy,
		"exitCode": exit,
		"id":       run.ID,
	}
}

// renderNotifyTemplate substitutes {{var}} placeholders (unknown vars → "").
func renderNotifyTemplate(tmpl string, vars map[string]string) string {
	out := tmpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func eventMatches(events []string, status string) bool {
	if len(events) == 0 {
		return true
	}
	for _, e := range events {
		if e == status {
			return true
		}
	}
	return false
}

// hasEvent reports whether a specific event is in the channel's filter.
func hasEvent(events []string, want string) bool {
	for _, e := range events {
		if e == want {
			return true
		}
	}
	return false
}
