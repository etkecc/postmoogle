package queue

import (
	"context"
	"net/url"
	"strconv"
)

// Add to queue
func (q *Queue) Add(ctx context.Context, id, from, to, data string, relayOverride ...*url.URL) error {
	itemkey := acQueueKey + "." + id
	relay := ""
	if len(relayOverride) > 0 {
		relay = relayOverride[0].String()
	}
	item := map[string]string{
		"attempts": "0",
		"relay":    relay,
		"data":     data,
		"from":     from,
		"to":       to,
		"id":       id,
	}

	q.mu.Lock(itemkey)
	defer q.mu.Unlock(itemkey)
	err := q.lp.SetAccountData(ctx, itemkey, item)
	if err != nil {
		q.log.Error().Err(err).Str("id", id).Msg("cannot enqueue email")
		return err
	}

	q.mu.Lock(acQueueKey)
	defer q.mu.Unlock(acQueueKey)
	queueIndex, err := q.lp.GetAccountData(ctx, acQueueKey)
	if err != nil {
		q.log.Error().Err(err).Msg("cannot get queue index")
		return err
	}
	queueIndex[id] = itemkey
	err = q.lp.SetAccountData(ctx, acQueueKey, queueIndex)
	if err != nil {
		q.log.Error().Err(err).Msg("cannot save queue index")
		return err
	}

	return nil
}

// Remove from queue
func (q *Queue) Remove(ctx context.Context, id string) error {
	index, err := q.lp.GetAccountData(ctx, acQueueKey)
	if err != nil {
		q.log.Error().Err(err).Msg("cannot get queue index")
		return err
	}
	itemkey := index[id]
	if itemkey == "" {
		itemkey = acQueueKey + "." + id
	}
	delete(index, id)
	err = q.lp.SetAccountData(ctx, acQueueKey, index)
	if err != nil {
		q.log.Error().Err(err).Msg("cannot update queue index")
		return err
	}

	q.mu.Lock(itemkey)
	defer q.mu.Unlock(itemkey)
	return q.lp.SetAccountData(ctx, itemkey, map[string]string{})
}

// try to send email
func (q *Queue) try(ctx context.Context, itemkey string, maxRetries int) bool {
	q.mu.Lock(itemkey)
	defer q.mu.Unlock(itemkey)

	item, err := q.lp.GetAccountData(ctx, itemkey)
	if err != nil {
		q.log.Error().Err(err).Str("id", itemkey).Msg("cannot retrieve a queue item")
		return false
	}
	q.log.Debug().Any("item", item).Msg("processing queue item")
	attempts, err := strconv.Atoi(item["attempts"])
	if err != nil {
		q.log.Error().Err(err).Str("id", itemkey).Msg("cannot parse attempts")
		return false
	}
	if attempts > maxRetries {
		return true
	}

	var relayOverride *url.URL
	if item["relay"] != "" {
		relayOverride, _ = url.Parse(item["relay"]) //nolint:errcheck // doesn't matter
	}

	err = q.sendmail(item["from"], item["to"], item["data"], relayOverride)
	if err == nil {
		q.log.Info().Str("id", itemkey).Msg("email from queue was delivered")
		return true
	}

	q.log.Info().Str("id", itemkey).Str("from", item["from"]).Str("to", item["to"]).Err(err).Msg("attempted to deliver email, but it's not ready yet")
	attempts++
	item["attempts"] = strconv.Itoa(attempts)
	err = q.lp.SetAccountData(ctx, itemkey, item)
	if err != nil {
		q.log.Error().Err(err).Str("id", itemkey).Msg("cannot update attempt count on email")
	}

	return false
}
