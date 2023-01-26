package queue

import (
	"strconv"
)

// Add to queue
func (q *Queue) Add(id, from, to, data string) error {
	itemkey := acQueueKey + "." + id
	item := map[string]string{
		"attempts": "0",
		"data":     data,
		"from":     from,
		"to":       to,
		"id":       id,
	}

	q.mu.Lock(itemkey)
	defer q.mu.Unlock(itemkey)
	err := q.lp.SetAccountData(itemkey, item)
	if err != nil {
		q.log.Error("cannot enqueue email id=%q: %v", id, err)
		return err
	}

	q.mu.Lock(acQueueKey)
	defer q.mu.Unlock(acQueueKey)
	queueIndex, err := q.lp.GetAccountData(acQueueKey)
	if err != nil {
		q.log.Error("cannot get queue index: %v", err)
		return err
	}
	queueIndex[id] = itemkey
	err = q.lp.SetAccountData(acQueueKey, queueIndex)
	if err != nil {
		q.log.Error("cannot save queue index: %v", err)
		return err
	}

	return nil
}

// Remove from queue
func (q *Queue) Remove(id string) error {
	index, err := q.lp.GetAccountData(acQueueKey)
	if err != nil {
		q.log.Error("cannot get queue index: %v", err)
		return err
	}
	itemkey := index[id]
	if itemkey == "" {
		itemkey = acQueueKey + "." + id
	}
	delete(index, id)
	err = q.lp.SetAccountData(acQueueKey, index)
	if err != nil {
		q.log.Error("cannot update queue index: %v", err)
		return err
	}

	q.mu.Lock(itemkey)
	defer q.mu.Unlock(itemkey)
	return q.lp.SetAccountData(itemkey, map[string]string{})
}

// try to send email
func (q *Queue) try(itemkey string, maxRetries int) bool {
	q.mu.Lock(itemkey)
	defer q.mu.Unlock(itemkey)

	item, err := q.lp.GetAccountData(itemkey)
	if err != nil {
		q.log.Error("cannot retrieve a queue item %q: %v", itemkey, err)
		return false
	}
	q.log.Debug("processing queue item %+v", item)
	attempts, err := strconv.Atoi(item["attempts"])
	if err != nil {
		q.log.Error("cannot parse attempts of %q: %v", itemkey, err)
		return false
	}
	if attempts > maxRetries {
		return true
	}

	err = q.sendmail(item["from"], item["to"], item["data"])
	if err == nil {
		q.log.Info("email %q from queue was delivered")
		return true
	}

	q.log.Info("attempted to deliver email id=%q, retry=%q, but it's not ready yet: %v", item["id"], item["attempts"], err)
	attempts++
	item["attempts"] = strconv.Itoa(attempts)
	err = q.lp.SetAccountData(itemkey, item)
	if err != nil {
		q.log.Error("cannot update attempt count on email %q: %v", itemkey, err)
	}

	return false
}
