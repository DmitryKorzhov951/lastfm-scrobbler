package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/history"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/lastfm"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/queue"
)

const maxScrobbleAge = 14 * 24 * time.Hour

const maxBatchesPerFlush = 4

const unknownLengthSeconds = 240

type submitter struct {
	queue  *queue.Queue
	hist   *history.Store
	log    *slog.Logger
	dryRun bool
	retry  time.Duration

	nowPlaying chan lastfm.Track
	scrobbles  chan lastfm.Scrobble
	flushReq   chan struct{}

	mu          sync.Mutex
	client      *lastfm.Client
	lastNP      lastfm.Track
	lastNPAt    time.Time
	failures    int
	nextAttempt time.Time
	lastErr     error
	authErr     bool
}

func newSubmitter(client *lastfm.Client, q *queue.Queue, hist *history.Store, retry time.Duration, dryRun bool, log *slog.Logger) *submitter {
	return &submitter{
		client:     client,
		queue:      q,
		hist:       hist,
		log:        log,
		dryRun:     dryRun,
		retry:      retry,
		nowPlaying: make(chan lastfm.Track, 8),
		scrobbles:  make(chan lastfm.Scrobble, 32),
		flushReq:   make(chan struct{}, 1),
	}
}

type sendStatus struct {
	LoggedIn bool
	Failing  bool
	AuthErr  bool
	LastErr  string
	NextTry  time.Time
}

func (s *submitter) Status() sendStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := sendStatus{
		LoggedIn: s.client != nil && s.client.SessionKey != "",
		Failing:  s.failures > 0,
		AuthErr:  s.authErr,
		NextTry:  s.nextAttempt,
	}
	if s.lastErr != nil {
		st.LastErr = s.lastErr.Error()
	}
	return st
}

func (s *submitter) SetClient(c *lastfm.Client) {
	s.mu.Lock()
	s.client = c
	s.failures, s.nextAttempt, s.lastErr, s.authErr = 0, time.Time{}, nil, false
	np, npAt := s.lastNP, s.lastNPAt
	s.mu.Unlock()
	s.RequestFlush()
	if c != nil && np.Valid() && time.Since(npAt) < 10*time.Minute {
		s.NowPlaying(np)
	}
}

func (s *submitter) RequestFlush() {
	select {
	case s.flushReq <- struct{}{}:
	default:
	}
}

func (s *submitter) currentClient() *lastfm.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil || s.client.SessionKey == "" {
		return nil
	}
	return s.client
}

func (s *submitter) NowPlaying(t lastfm.Track) {
	s.mu.Lock()
	s.lastNP, s.lastNPAt = t, time.Now()
	s.mu.Unlock()
	select {
	case s.nowPlaying <- t:
	default:
		s.log.Debug("пропущено уведомление «сейчас играет»", "track", t.String())
	}
}

func (s *submitter) Scrobble(sc lastfm.Scrobble) {
	select {
	case s.scrobbles <- sc:
	default:
		s.enqueue(sc)
	}
}

func (s *submitter) enqueue(sc lastfm.Scrobble) {
	it, err := s.queue.Push(sc)
	if err != nil {
		s.log.Error("не удалось сохранить скроббл в очередь", "err", err)
	}
	if s.hist == nil {
		return
	}
	secs := int(sc.Duration.Seconds())
	if secs <= 0 {
		secs = unknownLengthSeconds
	}
	err = s.hist.Add(history.Entry{
		ID:      it.ID,
		Artist:  sc.Artist,
		Title:   sc.Title,
		Album:   sc.Album,
		Seconds: secs,
		Started: sc.Timestamp,
		Status:  history.Queued,
	})
	if err != nil {
		s.log.Error("не удалось записать историю", "err", err)
	}
}

func (s *submitter) Run(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.finalFlush()
			return

		case t := <-s.nowPlaying:
			s.sendNowPlaying(ctx, t)

		case sc := <-s.scrobbles:
			s.enqueue(sc)
			s.flush(ctx, true)

		case <-s.flushReq:
			s.flush(ctx, true)

		case <-ticker.C:
			s.flush(ctx, false)
		}
	}
}

func (s *submitter) sendNowPlaying(ctx context.Context, t lastfm.Track) {
	if s.dryRun {
		s.log.Info("[dry-run] сейчас играет", "track", t.String())
		return
	}
	client := s.currentClient()
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if err := client.UpdateNowPlaying(ctx, t); err != nil {
		s.log.Warn("не удалось обновить «сейчас играет»", "track", t.String(), "err", err)
		return
	}
	s.log.Info("сейчас играет", "track", t.String())
}

func (s *submitter) flush(ctx context.Context, force bool) {
	if s.queue.Len() == 0 {
		return
	}
	s.mu.Lock()
	wait := !force && time.Now().Before(s.nextAttempt)
	s.mu.Unlock()
	if wait {
		return
	}
	if s.dryRun {
		items := s.queue.Items()
		for _, it := range items {
			s.log.Info("[dry-run] скроббл", "track", it.Scrobble.Track.String(),
				"timestamp", it.Scrobble.Timestamp.Format(time.RFC3339))
		}
		if _, err := s.queue.Remove(itemIDs(items)...); err != nil {
			s.log.Error("не удалось очистить очередь", "err", err)
		}
		return
	}
	client := s.currentClient()
	if client == nil {
		return
	}

	for i := 0; i < maxBatchesPerFlush && s.queue.Len() > 0; i++ {
		if ctx.Err() != nil {
			return
		}
		if !s.flushBatch(ctx, client) {
			return
		}
	}
}

func (s *submitter) flushBatch(ctx context.Context, client *lastfm.Client) bool {
	s.dropStale()

	items := s.queue.Head(lastfm.MaxBatch)
	if len(items) == 0 {
		return false
	}
	batch := make([]lastfm.Scrobble, 0, len(items))
	for _, it := range items {
		batch = append(batch, it.Scrobble)
	}
	ids := itemIDs(items)

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	res, err := client.Scrobble(callCtx, batch)
	cancel()

	if err == nil {
		s.mu.Lock()
		s.failures, s.nextAttempt, s.lastErr, s.authErr = 0, time.Time{}, nil, false
		s.mu.Unlock()
		if _, err := s.queue.Remove(ids...); err != nil {
			s.log.Error("не удалось обновить очередь", "err", err)
		}
		s.markSent(ids, res)
		s.log.Info("скробблы отправлены",
			"accepted", res.Accepted, "ignored", res.Ignored, "queue", s.queue.Len())
		for _, reason := range res.IgnoredReasons {
			s.log.Warn("Last.fm отклонил скроббл", "detail", reason)
		}
		return true
	}

	if !lastfm.Retryable(err) {
		s.log.Error("скробблы отброшены — Last.fm вернул постоянную ошибку",
			"count", len(batch), "err", err)
		if _, err := s.queue.Remove(ids...); err != nil {
			s.log.Error("не удалось обновить очередь", "err", err)
		}
		s.markIgnored(err.Error(), ids...)
		return true
	}

	if err := s.queue.MarkAttempts(ids...); err != nil {
		s.log.Error("не удалось обновить очередь", "err", err)
	}

	apiErr, isAPI := lastfm.AsError(err)
	needsAuth := isAPI && apiErr.NeedsAuth()
	s.mu.Lock()
	s.failures++
	delay := s.backoff()
	s.nextAttempt = time.Now().Add(delay)
	s.lastErr = err
	s.authErr = needsAuth
	s.mu.Unlock()

	if needsAuth {
		s.log.Error("Last.fm отклонил авторизацию — войдите в аккаунт заново",
			"err", err, "queue", s.queue.Len())
	} else {
		s.log.Warn("отправка не удалась, скробблы остаются в очереди",
			"err", err, "queue", s.queue.Len(), "retry_in", delay.Round(time.Second))
	}
	return false
}

func (s *submitter) markSent(ids []int64, res lastfm.ScrobbleResult) {
	if s.hist == nil {
		return
	}
	ignored := make(map[int]string, len(res.IgnoredIndex))
	for k, i := range res.IgnoredIndex {
		reason := ""
		if k < len(res.IgnoredReasons) {
			reason = res.IgnoredReasons[k]
		}
		ignored[i] = reason
	}
	sent := make([]int64, 0, len(ids))
	for i, id := range ids {
		if reason, ok := ignored[i]; ok {
			s.markIgnored(reason, id)
			continue
		}
		sent = append(sent, id)
	}
	if err := s.hist.SetStatus(history.Sent, "", sent...); err != nil {
		s.log.Error("не удалось записать историю", "err", err)
	}
}

func (s *submitter) markIgnored(reason string, ids ...int64) {
	if s.hist == nil {
		return
	}
	if err := s.hist.SetStatus(history.Ignored, reason, ids...); err != nil {
		s.log.Error("не удалось записать историю", "err", err)
	}
}

func (s *submitter) dropStale() {
	var stale []int64
	for _, it := range s.queue.Head(lastfm.MaxBatch) {
		if time.Since(it.Scrobble.Timestamp) < maxScrobbleAge {
			break
		}
		stale = append(stale, it.ID)
	}
	if len(stale) == 0 {
		return
	}
	s.log.Warn("скробблы отброшены по давности (старше 14 дней)", "count", len(stale))
	if _, err := s.queue.Remove(stale...); err != nil {
		s.log.Error("не удалось обновить очередь", "err", err)
	}
	s.markIgnored("старше 14 дней — Last.fm такие не принимает", stale...)
}

func (s *submitter) backoff() time.Duration {
	base := s.retry
	if base <= 0 {
		base = time.Minute
	}
	delay := base
	for i := 1; i < s.failures && delay < 30*time.Minute; i++ {
		delay *= 2
	}
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	return delay
}

func (s *submitter) finalFlush() {
	for {
		select {
		case sc := <-s.scrobbles:
			s.enqueue(sc)
			continue
		default:
		}
		break
	}
	if s.queue.Len() == 0 || s.dryRun {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s.flush(ctx, true)
	if n := s.queue.Len(); n > 0 {
		s.log.Info("скробблы остались в очереди и уйдут при следующем запуске", "count", n)
	}
}

func itemIDs(items []queue.Item) []int64 {
	ids := make([]int64, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}
