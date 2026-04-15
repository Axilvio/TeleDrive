package subscription

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Store interface {
	QueueSubscriptionReminders(ctx context.Context, daysBefore int) (int64, error)
	DisableExpiredUsers(ctx context.Context) (int64, error)
}

type Job struct {
	store Store
}

func NewJob(store Store) *Job {
	return &Job{store: store}
}

func (j *Job) Run(ctx context.Context) {
	j.process(ctx)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.process(ctx)
		}
	}
}

func (j *Job) process(ctx context.Context) {
	for _, d := range []int{3, 1} {
		rows, err := j.store.QueueSubscriptionReminders(ctx, d)
		if err != nil {
			log.Printf("queue reminder %dd failed: %v", d, err)
			continue
		}
		if rows > 0 {
			log.Printf("queued %d subscription reminders (%s)", rows, fmt.Sprintf("%dd", d))
		}
	}

	deactivated, err := j.store.DisableExpiredUsers(ctx)
	if err != nil {
		log.Printf("disable expired users failed: %v", err)
		return
	}
	if deactivated > 0 {
		log.Printf("deactivated %d expired configs", deactivated)
	}
}
