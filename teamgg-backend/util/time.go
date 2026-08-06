package util

import (
	log "github.com/shyunku-libraries/go-logger"
	"time"
)

type Timer struct {
	Name      string
	StartTime *time.Time
	EndTime   *time.Time
}

func NewTimer() *Timer {
	return &Timer{
		StartTime: nil,
		EndTime:   nil,
		Name:      "???",
	}
}

func NewTimerWithName(name string) *Timer {
	return &Timer{
		StartTime: nil,
		EndTime:   nil,
		Name:      name,
	}
}

func (t *Timer) Start() {
	now := time.Now()
	t.StartTime = &now
}

func (t *Timer) End() {
	now := time.Now()
	t.EndTime = &now
}

func (t *Timer) GetDuration() time.Duration {
	t.End()
	return t.EndTime.Sub(*t.StartTime)
}

func (t *Timer) GetDurationString() string {
	return t.GetDuration().String()
}

func (t *Timer) PrintDuration() {
	log.Infof("%s: %s", t.Name, t.GetDurationString())
}
