package worker

import (
	"context"
	"errors"
	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
	"github.com/kilan558/hllv-geofences/sync"
	"log/slog"
	"net"
	"slices"
	"strings"
	stdsync "sync"
	"time"
)

type Worker struct {
	pool                 *rconv2.ConnectionPool[*rconv2.HLLVConnection]
	l                    *slog.Logger
	c                    data.Server
	axisFences           []data.Fence
	alliesFences         []data.Fence
	punishAfterSeconds   time.Duration
	sessionTicker        *time.Ticker
	playerTicker         *time.Ticker
	punishTicker         *time.Ticker
	current              *rconv2.GetSessionInfoResponse
	lastSessionStatusLog time.Time
	outsidePlayers       sync.Map[string, outsidePlayer]
	firstCoord           sync.Map[string, *rconv2.GetPlayerPosition]

	restartCh     chan struct{}
	restartOnce   stdsync.Once
	timeoutStreak int
}

type outsidePlayer struct {
	Name            string
	LastGrid        rconv2.Grid
	FirstOutside    time.Time
	LastMessageTime time.Time // cooldown
	LastPunishTime  time.Time // cooldown
}

func (w *Worker) Host() string { return w.c.Host }

func NewWorker(l *slog.Logger, pool *rconv2.ConnectionPool[*rconv2.HLLVConnection], c data.Server) *Worker {
	punishAfterSeconds := 10
	if c.PunishAfterSeconds != nil {
		punishAfterSeconds = *c.PunishAfterSeconds
	}

	return &Worker{
		l:                  l,
		pool:               pool,
		punishAfterSeconds: time.Duration(punishAfterSeconds) * time.Second,
		c:                  c,
		sessionTicker:      time.NewTicker(1 * time.Second),
		playerTicker:       time.NewTicker(1000 * time.Millisecond),
		punishTicker:       time.NewTicker(time.Second),
		outsidePlayers:     sync.Map[string, outsidePlayer]{},
		firstCoord:         sync.Map[string, *rconv2.GetPlayerPosition]{},
		restartCh:          make(chan struct{}, 1), // buffered so it never blocks
	}
}

func (w *Worker) RestartSignal() <-chan struct{} { return w.restartCh }

func (w *Worker) logPlayerEvent(
	ctx context.Context,
	level slog.Level,
	event string,
	playerName string,
	playerID string,
	grid rconv2.Grid,
	err error,
) {
	attributes := []any{
		"player", playerName,
		"player_id", playerID,
		"grid", grid.String(),
	}
	if w.current != nil {
		attributes = append(
			attributes,
			"map", w.current.MapName,
			"mode", w.current.GameMode.String(),
		)
	}
	if err != nil {
		attributes = append(attributes, "error", err)
	}
	w.l.Log(ctx, level, event, attributes...)
}

func (w *Worker) logSessionStatus(now time.Time, force bool) {
	if w.current == nil || (!force && !w.lastSessionStatusLog.IsZero() && now.Sub(w.lastSessionStatusLog) < 20*time.Second) {
		return
	}
	w.lastSessionStatusLog = now
	w.l.Info(
		"session-status",
		"server_name", w.current.ServerName,
		"map", w.current.MapName,
		"mode", w.current.GameMode.String(),
		"player_count", w.current.PlayerCount,
		"max_player_count", w.current.MaxPlayerCount,
		"queue_count", w.current.QueueCount,
		"max_queue_count", w.current.MaxQueueCount,
	)
}

func (w *Worker) RefreshStatus(ctx context.Context) error {
	return w.populateSessionWithStatus(ctx, true)
}

func isTimeout(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "i/o timeout")
}

func isUnrecoverableRcon(err error) bool {
	return errors.Is(err, rconv2.ReconnectTriesExceeded)
}

func (w *Worker) handlePollError(event string, err error) (stop bool) {
	if err == nil {
		w.timeoutStreak = 0
		return false
	}
	if isTimeout(err) {
		w.timeoutStreak++
		w.l.Error(event+"-timeout", "streak", w.timeoutStreak, "error", err)
		if w.timeoutStreak >= 3 {
			w.requestRestart(event+"-timeout", err)
			return true
		}
		return false
	}
	if isUnrecoverableRcon(err) {
		w.l.Error(event, "error", err)
		w.requestRestart(event+"-reconnects-exhausted", err)
		return true
	}
	w.timeoutStreak = 0
	w.l.Error(event, "error", err)
	return false
}

func (w *Worker) requestRestart(reason string, err error) {
	w.restartOnce.Do(func() {
		w.l.Error("request-restart", "reason", reason, "error", err)
		select {
		case w.restartCh <- struct{}{}:
		default:
		}
	})
}

func (w *Worker) populateSessionWithRetry(ctx context.Context, maxAttempts int) error {
	backoff := 500 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := w.populateSession(ctx)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Only retry timeouts/transient network issues during startup.
		if !isTimeout(err) {
			return err
		}

		w.l.Warn("fetch-session-timeout", "attempt", attempt, "max", maxAttempts, "error", err)
		time.Sleep(backoff)

		if backoff < 10*time.Second {
			backoff *= 2
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
		}
	}

	return errors.New("fetch-session failed after retries")
}

func (w *Worker) Run(ctx context.Context) {
	// Startup: retry first session fetch, otherwise trigger restart.
	if err := w.populateSessionWithRetry(ctx, 8); err != nil {
		w.requestRestart("initial-fetch-session", err)
		return
	}

	go w.pollSession(ctx)
	go w.pollPlayers(ctx)
	go w.punishPlayers(ctx)
}

func (w *Worker) populateSession(ctx context.Context) error {
	return w.populateSessionWithStatus(ctx, false)
}

func (w *Worker) populateSessionWithStatus(ctx context.Context, forceStatusLog bool) error {
	return w.pool.WithConnection(ctx, func(c *rconv2.HLLVConnection) error {
		si, err := c.GetSessionInfo(ctx)
		if err != nil {
			return err
		}
		w.current = si
		w.logSessionStatus(time.Now(), forceStatusLog)
		w.axisFences = w.applicableFences(w.c.AxisFence)
		w.alliesFences = w.applicableFences(w.c.AlliesFence)
		return nil
	})
}

func (w *Worker) punishPlayers(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.punishTicker.Stop()
			return
		case <-w.punishTicker.C:
			w.outsidePlayers.Range(func(id string, o outsidePlayer) bool {
				if time.Since(o.FirstOutside) > w.punishAfterSeconds &&
					time.Since(o.FirstOutside) < w.punishAfterSeconds+5*time.Second {

					if time.Since(o.LastPunishTime) >= 15*time.Second {
						go w.punishPlayer(ctx, id, o)
					}
				}
				return true
			})
		}
	}
}

func (w *Worker) punishPlayer(ctx context.Context, id string, o outsidePlayer) {
	message := w.c.PunishMessage()
	w.l.Debug("punish-message-final", "message", message)

	err := w.pool.WithConnection(ctx, func(c *rconv2.HLLVConnection) error {
		return c.PunishPlayer(ctx, id, message)
	})
	if err != nil {
		w.logPlayerEvent(ctx, slog.LevelError, "punish-player", o.Name, id, o.LastGrid, err)
		return
	}

	w.logPlayerEvent(ctx, slog.LevelInfo, "punish-player", o.Name, id, o.LastGrid, nil)
	o.LastPunishTime = time.Now()
	w.outsidePlayers.Store(id, o)

	time.Sleep(5 * time.Second)
	w.outsidePlayers.Delete(id)
}

func (w *Worker) pollSession(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.sessionTicker.Stop()
			return
		case <-w.sessionTicker.C:
			if w.handlePollError("poll-session", w.populateSession(ctx)) {
				return
			}
		}
	}
}

func (w *Worker) pollPlayers(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.playerTicker.Stop()
			return
		case <-w.playerTicker.C:
			if len(w.alliesFences) == 0 && len(w.axisFences) == 0 {
				continue
			}

			err := w.pool.WithConnection(ctx, func(c *rconv2.HLLVConnection) error {
				players, err := c.GetPlayers(ctx)
				if err != nil {
					return err
				}

				for _, player := range players.Players {
					go w.checkPlayer(ctx, player)
				}

				w.firstCoord.Range(func(id string, p *rconv2.GetPlayerPosition) bool {
					for _, player := range players.Players {
						if player.Id == id {
							return true
						}
					}
					w.firstCoord.Delete(id)
					return true
				})

				return nil
			})

			if w.handlePollError("poll-players", err) {
				return
			}
		}
	}
}

func (w *Worker) checkPlayer(ctx context.Context, p rconv2.GetPlayersPlayer) {
	// Skip whitelisted players
	if slices.Contains(w.c.GetWhitelist(), p.Id) {
		w.outsidePlayers.Delete(p.Id)
		w.firstCoord.Delete(p.Id)
		return
	}

	position := p.Position.ToGetPlayerPosition()
	if !position.IsSpawned() {
		w.firstCoord.Store(p.Id, nil)
		return
	}

	if fp, ok := w.firstCoord.Load(p.Id); !ok {
		w.firstCoord.Store(p.Id, &position)
		return
	} else if fp != nil && position.Equal(*fp) {
		return
	} else {
		w.firstCoord.Store(p.Id, nil)
	}

	var fences []data.Fence
	switch hllvTeamSide(p.Team) {
	case teamAllies:
		fences = w.alliesFences
	case teamAxis:
		fences = w.axisFences
	}
	if len(fences) == 0 {
		return
	}

	g, err := gridForPosition(position, w.c.Grid)
	if err != nil {
		w.l.Warn("position-outside-grid", "player", p.Name, "map", w.current.MapName, "mode", w.current.GameMode.String(), "error", err)
		w.outsidePlayers.Delete(p.Id)
		return
	}
	for _, f := range fences {
		if f.Includes(g) {
			w.outsidePlayers.Delete(p.Id)
			return
		}
	}

	o, ok := w.outsidePlayers.Load(p.Id)
	if !ok {
		o = outsidePlayer{FirstOutside: time.Now(), Name: p.Name, LastGrid: g}
		w.outsidePlayers.Store(p.Id, o)
	} else {
		o.LastGrid = g
		w.outsidePlayers.Store(p.Id, o)
	}

	// message cooldown
	if !ok || time.Since(o.LastMessageTime) >= 30*time.Second {
		message := w.c.WarningMessage()
		w.l.Debug("warning-message-final", "message", message)

		err := w.pool.WithConnection(ctx, func(c *rconv2.HLLVConnection) error {
			return c.MessagePlayer(ctx, message, p.Id)
		})

		if err != nil {
			w.logPlayerEvent(ctx, slog.LevelError, "message-player-outside-fence", p.Name, p.Id, g, err)
		} else {
			w.logPlayerEvent(ctx, slog.LevelInfo, "player-outside-fence", p.Name, p.Id, g, nil)
			o.LastMessageTime = time.Now()
			w.outsidePlayers.Store(p.Id, o)
		}
	}
}

func (w *Worker) applicableFences(f []data.Fence) (v []data.Fence) {
	for _, fence := range f {
		if fence.Matches(w.current) {
			v = append(v, fence)
		}
	}
	return
}
