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
)

const BotShrineChannelId = "555128235869077506"
const ReminderSize = 256
const MaxRemindersCount = 5

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
			if (err != nil) {
				log.Println("Error:", err)
			}

			time.Sleep(1 * time.Minute)
		}
	}()
}

var Units = []string{"y", "d", "h", "m", "s"}

var UnitDurations = map[string]time.Duration{
	"s": time.Second,
	"m": time.Minute,
	"h": time.Hour,
	"d": 24 * time.Hour,
	"y": time.Duration(float64(365) * float64(24 * time.Hour)),
}

func DurationToString(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	neg := d < 0
	if neg {
		d = -d
	}

	var parts []string
	rem := d

	for _, unit := range Units {
		unitDur := UnitDurations[unit]
		if rem >= unitDur {
			n := rem / unitDur
			rem = rem % unitDur
			parts = append(parts, fmt.Sprintf("%d%s", n, unit))
		}
	}

	if len(parts) == 0 {
		parts = append(parts, "0s")
	}

	res := strings.Join(parts, "")
	if neg {
		res = "-" + res
	}
	return res
}

func ParseDurationStr(durationStr string) (time.Duration, error) {
	delay := time.Duration(0)

	for _, match := range ReminderDurationRegexp.FindAllStringSubmatch(durationStr, -1) {
		ammount, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			log.Println("Reminder duration parsing: ", err)
			return 0, fmt.Errorf("Delay ammount overflows.")
		}
		unit := match[2]

		d, ok := MulDurationSafe(ammount, UnitDurations[unit])
		if !ok {
			return 0, fmt.Errorf("Delay ammount overflows.")
		}

		delay, ok = AddDurationSafe(delay, d)
		if !ok {
			return 0, fmt.Errorf("Duration specified caused an overflow.")
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
	rows, err := db.Query("select id, user_id, message, remind_at from Reminders where remind_at < $1", time.Now())
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
	rows, err := db.Query("select id, user_id, message, remind_at from Reminders where user_id = $1 order by remind_at asc", userId)
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
	err := db.QueryRow("SELECT count(*) FROM Reminders WHERE user_id = $1", userId).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func InsertReminder(db *sql.DB, reminder Reminder) error {
	_, err := db.Exec("INSERT INTO Reminders (user_id, message, remind_at) VALUES ($1, $2, $3);", reminder.UserId, reminder.Message, reminder.RemindAt);
	return err;
}

func ValidateReminder(r Reminder) error {
	delay := r.RemindAt.Sub(time.Now())

	if (delay < 1*time.Minute) {
		return fmt.Errorf("Delay specified is too small")
	}

	if (len([]rune(r.Message)) > ReminderSize) {
		return fmt.Errorf("Reminder message must be max %v characters long", ReminderSize)
	}

	return nil
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
