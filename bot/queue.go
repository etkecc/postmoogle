package bot

import (
	"strconv"
)

const (
	defaultQueueBatch   = 1
	defaultQueueRetries = 3
)

// ProcessQueue starts queue processing
func (b *Bot) ProcessQueue() {
	b.log.Debug("staring queue processing...")
	cfg := b.getBotSettings()

	batchSize := cfg.QueueBatch()
	if batchSize == 0 {
		batchSize = defaultQueueBatch
	}

	retries := cfg.QueueRetries()
	if retries == 0 {
		retries = defaultQueueRetries
	}

	b.popqueue(batchSize, retries)
	b.log.Debug("ended queue processing")
}

// popqueue gets emails from queue and tries to send them,
// if an email was sent successfully - it will be removed from queue
func (b *Bot) popqueue(batchSize, maxTries int) {
	b.lock(acQueueKey)
	defer b.unlock(acQueueKey)
	index, err := b.lp.GetAccountData(acQueueKey)
	if err != nil {
		b.log.Error("cannot get queue index: %v", err)
	}

	i := 0
	for id, itemkey := range index {
		if i > batchSize {
			b.log.Debug("finished re-deliveries from queue")
			return
		}
		if dequeue := b.processQueueItem(itemkey, maxTries); dequeue {
			b.log.Debug("email %s has been delivered", id)
			err = b.dequeueEmail(id)
			if err != nil {
				b.log.Error("cannot dequeue email %s: %v", id, err)
			}
		}
		i++
	}
}

// processQueueItem tries to process an item from queue
// returns should the item be dequeued or not
func (b *Bot) processQueueItem(itemkey string, maxRetries int) bool {
	b.lock(itemkey)
	defer b.unlock(itemkey)

	item, err := b.lp.GetAccountData(itemkey)
	if err != nil {
		b.log.Error("cannot retrieve a queue item %s: %v", itemkey, err)
		return false
	}
	b.log.Debug("processing queue item %+v", item)
	attempts, err := strconv.Atoi(item["attempts"])
	if err != nil {
		b.log.Error("cannot parse attempts of %s: %v", itemkey, err)
		return false
	}
	if attempts > maxRetries {
		return true
	}

	err = b.sendmail(item["from"], item["to"], item["data"])
	if err == nil {
		b.log.Debug("email %s from queue was delivered")
		return true
	}

	b.log.Debug("attempted to deliver email id=%s, retry=%s, but it's not ready yet: %v", item["id"], item["attempts"], err)
	attempts++
	item["attempts"] = strconv.Itoa(attempts)
	err = b.lp.SetAccountData(itemkey, item)
	if err != nil {
		b.log.Error("cannot update attempt count on email %s: %v", itemkey, err)
	}

	return false
}

// enqueueEmail adds an email to the queue
func (b *Bot) enqueueEmail(id, from, to, data string) error {
	itemkey := acQueueKey + "." + id
	item := map[string]string{
		"attempts": "0",
		"data":     data,
		"from":     from,
		"to":       to,
		"id":       id,
	}

	b.lock(itemkey)
	defer b.unlock(itemkey)
	err := b.lp.SetAccountData(itemkey, item)
	if err != nil {
		b.log.Error("cannot enqueue email id=%s: %v", id, err)
		return err
	}

	b.lock(acQueueKey)
	defer b.unlock(acQueueKey)
	queueIndex, err := b.lp.GetAccountData(acQueueKey)
	if err != nil {
		b.log.Error("cannot get queue index: %v", err)
		return err
	}
	queueIndex[id] = itemkey
	err = b.lp.SetAccountData(acQueueKey, queueIndex)
	if err != nil {
		b.log.Error("cannot save queue index: %v", err)
		return err
	}

	return nil
}

// dequeueEmail removes an email from the queue
func (b *Bot) dequeueEmail(id string) error {
	index, err := b.lp.GetAccountData(acQueueKey)
	if err != nil {
		b.log.Error("cannot get queue index: %v", err)
		return err
	}
	itemkey := index[id]
	if itemkey == "" {
		itemkey = acQueueKey + "." + id
	}
	delete(index, id)
	err = b.lp.SetAccountData(acQueueKey, index)
	if err != nil {
		b.log.Error("cannot update queue index: %v", err)
		return err
	}

	b.lock(itemkey)
	defer b.unlock(itemkey)
	return b.lp.SetAccountData(itemkey, nil)
}
