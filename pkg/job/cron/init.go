package cron

import "context"

func InitCrontab(ctx context.Context) *Cron {
	scheduler := New()
	scheduler.Start(ctx)
	return scheduler
}
