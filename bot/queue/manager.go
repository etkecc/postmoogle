package queue

import (
	"context"

	"github.com/rs/zerolog"
	"gitlab.com/etke.cc/linkpearl"

	"gitlab.com/etke.cc/postmoogle/bot/config"
	"gitlab.com/etke.cc/postmoogle/utils"
)

const (
	acQueueKey          = "cc.etke.postmoogle.mailqueue"
	defaultQueueBatch   = 10
	defaultQueueRetries = 100
)

// Queue manager
type Queue struct {
	mu       utils.Mutex
	lp       *linkpearl.Linkpearl
	cfg      *config.Manager
	log      *zerolog.Logger
	sendmail func(string, string, string) error
}

// New queue
func New(lp *linkpearl.Linkpearl, cfg *config.Manager, log *zerolog.Logger) *Queue {
	return &Queue{
		mu:  utils.Mutex{},
		lp:  lp,
		cfg: cfg,
		log: log,
	}
}

// SetSendmail func
func (q *Queue) SetSendmail(function func(string, string, string) error) {
	q.sendmail = function
}

// Process queue
func (q *Queue) Process() {
	q.log.Debug().Msg("staring queue processing...")
	ctx := context.Background()
	cfg := q.cfg.GetBot(ctx)

	batchSize := cfg.QueueBatch()
	if batchSize == 0 {
		batchSize = defaultQueueBatch
	}

	maxRetries := cfg.QueueRetries()
	if maxRetries == 0 {
		maxRetries = defaultQueueRetries
	}

	q.mu.Lock(acQueueKey)
	defer q.mu.Unlock(acQueueKey)
	index, err := q.lp.GetAccountData(ctx, acQueueKey)
	if err != nil {
		q.log.Error().Err(err).Msg("cannot get queue index")
	}

	i := 0
	for id, itemkey := range index {
		if i > batchSize {
			q.log.Debug().Msg("finished re-deliveries from queue")
			return
		}
		if dequeue := q.try(ctx, itemkey, maxRetries); dequeue {
			q.log.Info().Str("id", id).Msg("email has been delivered")
			err = q.Remove(ctx, id)
			if err != nil {
				q.log.Error().Err(err).Str("id", id).Msg("cannot dequeue email")
			}
		}
		i++
	}
	q.log.Debug().Msg("ended queue processing")
}
