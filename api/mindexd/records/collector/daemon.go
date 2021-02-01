package collector

import (
	"context"
	"fmt"
	"sync"
)

func (c *Collector) collectTargets(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(c.cfg.pows))

	for _, source := range c.cfg.pows {
		go func() {
			defer wg.Done()
			if err := c.collectNewRecords(source); err != nil {
				log.Errorf("collecting records from %s: %s", source.Name, err)
			}
		}()
	}

	wg.Wait()
}

func (c *Collector) collectNewRecords(t PowTarget) error {
	log.Debugf("collecting new records from %s", t.Name)

	lastSince, err := c.getLastRecordTimestamp(t.Name)
	if err != nil {
		return fmt.Errorf("get last record timestamp: %s", err)
	}

	var count int
	for {
		count, lastSince, err = c.fetchRecords(lastSince, c.cfg.fetchLimit)
		if err != nil {
			return fmt.Errorf("fetching records: %s", err)
		}

		// If we fetched less than limit, then there're no
		// more records to fetch.
		if count < c.cfg.fetchLimit {
			break
		}
	}

	return nil
}
