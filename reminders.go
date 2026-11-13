package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/yuluo-yx/mory/internal/blocks"
)

type reminder struct {
	Key, Title string
	At         time.Time
}

func documentReminders(source, format string) []reminder {
	d, err := blocks.Parse(source, format)
	if err != nil {
		return nil
	}
	var out []reminder
	d.Root.Walk(func(n *blocks.Node) {
		var at time.Time
		title := "Reminder"
		switch n.Kind {
		case "countdown":
			if n.Attr("notify") == "false" {
				return
			}
			start, err := strconv.ParseInt(n.Attr("start-time"), 10, 64)
			if err != nil || start <= 0 {
				return
			}
			at = time.UnixMilli(start).Add(time.Duration(n.Int("duration", 300)) * time.Second)
			title = "Timer finished"
		case "time":
			value := strings.TrimSpace(n.Attr("date") + " " + n.Attr("time"))
			for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
				if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
					at = t
					break
				}
			}
			if at.IsZero() {
				if millis, err := strconv.ParseInt(n.Attr("time"), 10, 64); err == nil {
					at = time.UnixMilli(millis)
				}
			}
		default:
			return
		}
		if !at.IsZero() {
			out = append(out, reminder{Key: fmt.Sprintf("%s:%d:%d", n.Kind, at.UnixMilli(), len(out)), Title: title, At: at})
		}
	})
	return out
}

func (a *app) checkReminders(now time.Time) {
	if a.reminders == nil {
		a.reminders = map[string][]reminder{}
		a.reminderSources = map[string]string{}
		a.notified = map[string]bool{}
	}
	for _, n := range a.lib.Notes {
		if n.Deleted {
			continue
		}
		if a.reminderSources[n.ID] != n.Source {
			a.reminders[n.ID] = documentReminders(n.Source, n.Format)
			a.reminderSources[n.ID] = n.Source
		}
		for _, r := range a.reminders[n.ID] {
			key := n.ID + ":" + r.Key
			if a.notified[key] || r.At.After(now) || now.Sub(r.At) > 24*time.Hour {
				continue
			}
			a.notified[key] = true
			a.status = r.Title + " · " + a.displayTitle(n.Title)
			if a.win != nil {
				notification := mygo.NewNotification(mygo.NotificationOptions{ID: key, Title: r.Title, Body: a.displayTitle(n.Title)})
				note := n
				notification.OnClick(func() { a.win.Update(func() { a.openNote(note); a.win.Show(); a.win.Focus() }) })
				go func() {
					err := notification.Show()
					if err != nil {
						a.win.Update(func() { a.status = "Reminder: " + a.displayTitle(note.Title) })
					}
				}()
			}
		}
	}
}
