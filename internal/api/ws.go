package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nikiv/ansible-ui/internal/model"
)

const (
	wsWriteWait = 10 * time.Second
	wsPingEvery = 25 * time.Second
)

// handleEventsWS streams run lifecycle events to the browser.
func (s *Server) handleEventsWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch, unsub := s.events.Subscribe()
	defer unsub()

	gone := make(chan struct{})
	go func() {
		defer close(gone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(wsPingEvery)
	defer ticker.Stop()
	for {
		select {
		case b, ok := <-ch:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
				return
			}
		case <-gone:
			return
		}
	}
}

// handleRunWS streams a single run's terminal output to xterm.js. Terminal
// bytes are sent as binary frames; status/lifecycle as text JSON frames.
// Control frames (resize/cancel) flow the other way as text JSON.
func (s *Server) handleRunWS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		s.handleStoreErr(w, err) // 404 before upgrade
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	sendMeta(conn, map[string]any{"type": "status", "status": run.Status, "run": run})

	snapshot, ch, unsub, active := s.manager.Subscribe(id)
	if !active {
		if !model.IsTerminalStatus(run.Status) {
			// In progress but not streaming on THIS replica → it's executing on
			// another replica. Tail the Postgres-persisted output (flushed ~1/s by
			// the owning replica) so the live terminal works from any replica.
			s.tailRunWS(r.Context(), conn, id)
			return
		}
		// Finished run: replay the stored output, then close.
		if out, oerr := s.store.GetRunOutput(r.Context(), id); oerr == nil && len(out) > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			_ = conn.WriteMessage(websocket.BinaryMessage, out)
		}
		sendMeta(conn, map[string]any{"type": "end", "status": run.Status, "run": run})
		return
	}
	defer unsub()

	// Replay output accumulated before we subscribed.
	if len(snapshot) > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
		_ = conn.WriteMessage(websocket.BinaryMessage, snapshot)
	}

	// Browser → server control frames.
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		for {
			mt, data, rerr := conn.ReadMessage()
			if rerr != nil {
				return
			}
			if mt != websocket.TextMessage {
				continue
			}
			var f model.Frame
			if json.Unmarshal(data, &f) != nil {
				continue
			}
			switch f.Type {
			case model.FrameResize:
				s.manager.Control(id, model.Frame{Type: model.FrameResize, Cols: f.Cols, Rows: f.Rows})
			case model.FrameCancel:
				s.manager.Control(id, model.Frame{Type: model.FrameCancel})
			}
		}
	}()

	ticker := time.NewTicker(wsPingEvery)
	defer ticker.Stop()
	for {
		select {
		case b, ok := <-ch:
			if !ok {
				// Run finished: report the final state.
				final, ferr := s.store.GetRun(r.Context(), id)
				if ferr == nil {
					sendMeta(conn, map[string]any{"type": "end", "status": final.Status, "run": final})
				}
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
				return
			}
		case <-gone:
			return
		}
	}
}

// tailRunWS streams a run that is executing on ANOTHER replica by polling the
// Postgres-persisted output (the owning replica flushes it ~1/s) and pushing new
// bytes until the run reaches a terminal status. This makes /ws/runs/{id} work
// from any replica — no sticky ingress or pub/sub bus needed (true active-active).
// A cancel control frame from the browser is relayed cross-replica via the DB.
func (s *Server) tailRunWS(ctx context.Context, conn *websocket.Conn, id string) {
	sent := 0
	flush := func() bool { // false = write error (client gone)
		out, err := s.store.GetRunOutput(ctx, id)
		if err == nil && len(out) > sent {
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if werr := conn.WriteMessage(websocket.BinaryMessage, out[sent:]); werr != nil {
				return false
			}
			sent = len(out)
		}
		return true
	}

	// Browser → server: a viewer on this replica can still cancel; relay via the DB.
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		for {
			mt, data, rerr := conn.ReadMessage()
			if rerr != nil {
				return
			}
			if mt != websocket.TextMessage {
				continue
			}
			var f model.Frame
			if json.Unmarshal(data, &f) != nil {
				continue
			}
			if f.Type == model.FrameCancel {
				_ = s.store.RequestCancel(ctx, id) // the owning replica picks it up
			}
			// resize is cosmetic for a remote viewer — ignore.
		}
	}()

	if !flush() {
		return
	}
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	ping := time.NewTicker(wsPingEvery)
	defer ping.Stop()
	for {
		select {
		case <-poll.C:
			if !flush() {
				return
			}
			run, err := s.store.GetRun(ctx, id)
			if err == nil && model.IsTerminalStatus(run.Status) {
				_ = flush() // final delta
				sendMeta(conn, map[string]any{"type": "end", "status": run.Status, "run": run})
				return
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
				return
			}
		case <-gone:
			return
		}
	}
}

func sendMeta(conn *websocket.Conn, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	_ = conn.WriteMessage(websocket.TextMessage, b)
}
