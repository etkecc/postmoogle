package queue

import (
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/linkpearl"

	"gitlab.com/etke.cc/postmoogle/bot/config"
	"gitlab.com/etke.cc/postmoogle/utils"
)

const (
	acQueueKey          = "cc.etke.postmoogle.mailqueue"
	defaultQueueBatch   = 1
	defaultQueueRetries = 3
)

// Queue manager
type Queue struct {
	mu       utils.Mutex
	lp       *linkpearl.Linkpearl
	cfg      *config.Manager
	log      *logger.Logger
	sendmail func(string, string, string) error
}

// New queue
func New(lp *linkpearl.Linkpearl, cfg *config.Manager, log *logger.Logger) *Queue {
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
	q.log.Debug("staring queue processing...")
	cfg := q.cfg.GetBot()

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
	index, err := q.lp.GetAccountData(acQueueKey)
	if err != nil {
		q.log.Error("cannot get queue index: %v", err)
	}

	i := 0
	for id, itemkey := range index {
		if i > batchSize {
			q.log.Debug("finished re-deliveries from queue")
			return
		}
		if dequeue := q.try(itemkey, maxRetries); dequeue {
			q.log.Debug("email %q has been delivered", id)
			err = q.Remove(id)
			if err != nil {
				q.log.Error("cannot dequeue email %q: %v", id, err)
			}
		}
		i++
	}
	q.log.Debug("ended queue processing")
}
