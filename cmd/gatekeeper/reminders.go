package main

import (
	"context"
	"sync"
	"time"
	"log"
	"math"
)

type Reminder struct {
	Message string
	Delay   time.Duration
}

var UnitDurations = map[string]time.Duration{
	"s": time.Second,
	"m": time.Minute,
	"h": time.Hour,
	"d": 24 * time.Hour,
	"y": time.Duration(float64(365.2425) * float64(24 * time.Hour)),
}

type UserId = string

var mutex = &sync.Mutex{}
var reminders = map[UserId]context.CancelFunc{}

func SetReminder(env CommandEnvironment, r Reminder) {
	if !validateReminder(env, r) {
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	cancelOldReminder, ok := reminders[env.AuthorUserId()]
	if ok {
		cancelOldReminder()
	}

	reminders[env.AuthorUserId()] = cancel

	go func() {
		timer := time.NewTimer(r.Delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			env.SendMessage(env.AtAuthor() + " " + r.Message + "\n")
		case <-ctx.Done():
			env.SendMessage(env.AtAuthor() + " Old reminder has been canceled: '" + r.Message + "'")
		}
	}()

	env.SendMessage(env.AtAuthor() + " Reminder has been successfully set to fire in " + r.Delay.String() + ".")
}

func validateReminder(env CommandEnvironment, r Reminder) bool {
	if (r.Delay < 1 * time.Second) {
		log.Println("Smol reminder delay: " + r.Delay.String())
		env.SendMessage(env.AtAuthor() + " Delay specified has an unexpected duration, check logs." + "\n")
		return false
	}

	if (env.IsAuthorAdmin()) {
		return true
	}

	if (len(r.Message) > 255) {
		env.SendMessage(env.AtAuthor() + " Message body must be under 255 characters long" + "\n")
		return false
	}

	return true
}

func AddDurationSafe(a, b time.Duration) (time.Duration, bool) {
    if b > 0 && a > time.Duration(math.MaxInt64)-b {
        return 0, false
    }
    if b < 0 && a < time.Duration(math.MinInt64)-b {
        return 0, false
    }
    return a + b, true
}

func MulDurationSafe(ammount int64, d time.Duration) (time.Duration, bool) {
    if d == 0 || ammount == 0 {
        return 0, true
    }
    if ammount > 0 {
        if d > time.Duration(math.MaxInt64)/time.Duration(ammount) {
            return 0, false
        }
    } else {
        if d < time.Duration(math.MinInt64)/time.Duration(ammount) {
            return 0, false
        }
    }
    return d * time.Duration(ammount), true
}
