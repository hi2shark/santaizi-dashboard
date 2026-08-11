package singleton

import (
	"github.com/robfig/cron/v3"
)

var Cron *cron.Cron

// InitScheduler starts the internal scheduler used by service monitoring,
// rollups and retention. It intentionally has no user-defined command jobs.
func InitScheduler() {
	Cron = cron.New(cron.WithSeconds(), cron.WithLocation(Loc))
	Cron.Start()
}
