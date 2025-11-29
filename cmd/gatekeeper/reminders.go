package main

import (
	"time"
	"log"
	"math"
	"database/sql"
	"github.com/bwmarrin/discordgo"
	"github.com/lib/pq"
	"fmt"
	"strconv"
	"strings"
	"errors"
)

const BotShrineChannelId = "555128235869077506"
const ReminderSize = 256
const MaxRemindersCount = 5
const MinimumReminderDelay = 1 * time.Minute
const ReminderPoolInterval = MinimumReminderDelay

type ReminderDelay struct {
	Seconds int
	Minutes int
	Hours   int
	Days    int
	Months  int
	Years   int
}

func AddDelayToTimestamp(t1 time.Time, delay ReminderDelay) (time.Time, error) {
	t2 := t1.Add(time.Duration(delay.Seconds) * time.Second)
	t2 = t2.Add(time.Duration(delay.Minutes) * time.Minute)
	t2 = t2.Add(time.Duration(delay.Hours) * time.Hour)
	t2 = t2.AddDate(delay.Years, delay.Months, delay.Days)

	if t2.Before(t1) {
		return time.Time{}, errors.New("Time overflow")
	}

	return t2, nil
}

const (
	SecondsCode string = "s"
	MinutesCode = "m"
	HoursCode = "h"
	DaysCode = "d"
	MonthsCode = "M"
	YearsCode = "y"
)

type Reminder struct {
	Id       int64
	UserId   string
	Message  string
	RemindAt time.Time
}

type DiscordSession interface {
	ChannelMessageSend(channelID string, content string) (*discordgo.Message, error)
}

func PollOverdueReminders(db *sql.DB, dg DiscordSession) {
	go func() {
		for {
			reminders, err := QueryOverdueReminders(db)
			if err != nil {
				log.Println("Error querying overdue reminders", err)
				continue
			}

			successfullyFiredReminders := []int64{}
			for _, reminder := range reminders {
				_, err := dg.ChannelMessageSend(BotShrineChannelId, AtID(reminder.UserId) + " " + reminder.Message)
				if err != nil {
					log.Printf("Error during sending discord message\n", err)
					continue
				}
				successfullyFiredReminders = append(successfullyFiredReminders, reminder.Id)
			}

			_, err = db.Exec("DELETE FROM Reminders WHERE id = ANY($1);", pq.Array(successfullyFiredReminders));
			if err != nil {
				log.Println("Error:", err)
			}

			time.Sleep(ReminderPoolInterval)
		}
	}()
}

var Units = []string{"y", "M", "d", "h", "m", "s"}

func DurationToString(from, to time.Time) string {
	if from.Equal(to) {
		return "0s"
	}

	var parts []string
	cur := from

	year := 0
	for next := cur.AddDate(1, 0, 0); !next.After(to); next = cur.AddDate(1, 0, 0) {
		cur = next
		year++
	}
	if year > 0 {
		parts = append(parts, fmt.Sprintf("%dy", year))
	}

	month := 0
	for next := cur.AddDate(0, 1, 0); !next.After(to); next = cur.AddDate(0, 1, 0) {
		cur = next
		month++
	}
	if month > 0 {
		parts = append(parts, fmt.Sprintf("%dM", month))
	}

	rem := to.Sub(cur)

	day := rem / (24 * time.Hour)
	if day > 0 {
		parts = append(parts, fmt.Sprintf("%dd", day))
		rem -= day * 24 * time.Hour
	}

	hour := rem / time.Hour
	if hour > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hour))
		rem -= hour * time.Hour
	}

	min := rem / time.Minute
	if min > 0 {
		parts = append(parts, fmt.Sprintf("%dm", min))
		rem -= min * time.Minute
	}

	sec := rem / time.Second
	if sec > 0 {
		parts = append(parts, fmt.Sprintf("%ds", sec))
	}

	if len(parts) == 0 {
		return "0s"
	}

	return strings.Join(parts, "")
}

func ParseReminderDelayStr(durationStr string) (ReminderDelay, error) {
	delay := ReminderDelay{}

	for _, match := range ReminderDurationRegexp.FindAllStringSubmatch(durationStr, -1) {
		ammount, err := strconv.ParseInt(match[1], 10, 32)
		if err != nil {
			log.Println("Error parsing reminder duration: ", err)
			return ReminderDelay{}, fmt.Errorf("Delay ammount overflows.")
		}
		unit := match[2]

		switch unit {
		case SecondsCode:
			delay.Seconds = int(ammount)
		case MinutesCode:
			delay.Minutes = int(ammount)
		case HoursCode:
			delay.Hours = int(ammount)
		case DaysCode:
			delay.Days = int(ammount)
		case MonthsCode:
			delay.Months = int(ammount)
		case YearsCode:
			delay.Years = int(ammount)
		}
	}

	return delay, nil
}

func SetReminder(db *sql.DB, r Reminder) error {
	if err := ValidateReminder(r); err != nil {
		return err
	}

	c, err := CountUserReminders(db, r.UserId)
	if err != nil {
		log.Println(err)
		return fmt.Errorf("There has been an error adding the reminder, please ask the admin to check the logs.")
	}

	if c >= MaxRemindersCount {
		return fmt.Errorf("You have exeeded your max reminders count (you may have %v).", MaxRemindersCount)
	}

	if err := InsertReminder(db, r); err != nil {
		log.Println(err)
		return fmt.Errorf("There has been an error adding the reminder, please ask the admin to check the logs.")
	}

	return nil
}

func DelReminder(db *sql.DB, id int64) error {
	res, err := db.Exec("DELETE FROM Reminders WHERE id = $1", id)
	if err != nil {
		log.Println(err)
		return fmt.Errorf("Something went wrong, please ask the admin to check the logs.")
	}

	affected, err := res.RowsAffected()
	if err != nil {
		log.Println(err)
		return fmt.Errorf("There has been an error deleting the reminder, please ask the admin to check the logs.")
	}
	if affected == 0 {
		return fmt.Errorf("Reminder not found")
	}
	return nil
}

func QueryOverdueReminders(db *sql.DB) ([]Reminder, error) {
	rows, err := db.Query("select id, discord_user_id, message, remind_at from Reminders where remind_at < $1", time.Now())
	if err != nil {
		return nil, err
	}

	reminders := []Reminder{}
	for rows.Next() {
		r := Reminder{}
    	if err := rows.Scan(&r.Id, &r.UserId, &r.Message, &r.RemindAt); err != nil {
			return nil, err
		}
		reminders = append(reminders, r)
	}

	return reminders, nil
}

func QueryUserReminders(userId string, db *sql.DB) ([]Reminder, error) {
	rows, err := db.Query("select id, discord_user_id, message, remind_at from Reminders where discord_user_id = $1 order by remind_at asc", userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reminders := []Reminder{}
	for rows.Next() {
		r := Reminder{}
		if err := rows.Scan(&r.Id, &r.UserId, &r.Message, &r.RemindAt); err != nil {
			return nil, err
		}
		reminders = append(reminders, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return reminders, nil
}

func CountUserReminders(db *sql.DB, userId string) (int, error) {
	count := int(0)
	err := db.QueryRow("SELECT count(*) FROM Reminders WHERE discord_user_id = $1", userId).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func InsertReminder(db *sql.DB, reminder Reminder) error {
	_, err := db.Exec("INSERT INTO Reminders (discord_user_id, message, remind_at) VALUES ($1, $2, $3);", reminder.UserId, reminder.Message, reminder.RemindAt);
	return err;
}

func ValidateReminder(r Reminder) error {
	delay := r.RemindAt.Sub(time.Now())

	max := time.Now().AddDate(290000, 12, 0)
	if r.RemindAt.After(max) {
		return errors.New("Timestamp out of range, max is ~290000 years")
	}

	if delay < MinimumReminderDelay {
		return fmt.Errorf("Delay specified is too small")
	}

	if len([]rune(r.Message)) > ReminderSize {
		return fmt.Errorf("Reminder message must be max %v characters long", ReminderSize)
	}

	return nil
}

func MulDurationSafe(ammount int, d time.Duration) (time.Duration, bool) {
    if d == 0 || ammount == 0 {
        return 0, true
    }
    if ammount > 0 {
        if d > time.Duration(math.MaxInt)/time.Duration(ammount) {
            return 0, false
        }
    } else {
        if d < time.Duration(math.MinInt)/time.Duration(ammount) {
            return 0, false
        }
    }
    return d * time.Duration(ammount), true
}
