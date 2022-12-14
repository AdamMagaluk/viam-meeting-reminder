package main

import (
	"context"
	"flag"
	"time"

	"adammagaluk.io/meeting-reminder/calendar"
	"adammagaluk.io/meeting-reminder/device"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

const (
	emailPullInterval    = 5 * time.Second
	notificationDuration = 30 * time.Second
)

var calendarID = flag.String("calendar", "primary", `calendar ID to use defaults to "primary"`)
var debug = flag.Bool("debug", false, `run in debug mode`)
var noRobot = flag.Bool("no-robot", false, `run without a robot`)
var remindEnd = flag.Bool("remind-end", false, `meeting remind at the end of events.`)
var reminderTime = flag.Duration("reminder-time", 2*time.Minute, `notify x time before the start/end of an event`)
var extraCalendarQuery = flag.String("calendar-query", "", `extra query to send when retrieving the calendar eg. location=NYC-1900-6-E7`)

func main() {
	flag.Parse()

	var logger golog.Logger
	if *debug {
		logger = golog.NewDebugLogger("meeting-reminder")
		logger.Debug("test")
	} else {
		logger = golog.NewLogger("meeting-reminder")
	}

	notifiedEvents := make(map[string]bool)

	var err error
	var reminderDevice *device.ReminderDevice
	if !*noRobot {
		reminderDevice, err = device.NewDevice("./robot-config.json")
		if err != nil {
			logger.Fatal(errors.Wrap(err, "unable to connect to robot"))
		}
		defer reminderDevice.Close(context.Background())
	}

	calendarClient, err := calendar.NewClient(context.Background(), "./calendar_oauth_creds.json", *calendarID, *extraCalendarQuery, logger)
	if err != nil {
		logger.Fatal(err)
	}

	notify := func(event *calendar.Event) {
		notifiedEvents[event.ID] = true
		logger.Infof("Notifying for event %s", event.Title)

		if *noRobot {
			return
		}

		ctxWithDeadline, cancelNotification := context.WithDeadline(context.TODO(), time.Now().Add(notificationDuration))
		reminderDevice.StartNotification(ctxWithDeadline)

		callback := reminderDevice.NotifyOnceIfButtonPress(ctxWithDeadline)

		select {
		case <-callback:
			logger.Info("Silence event")
			// silence button pressed
			break
		case <-ctxWithDeadline.Done():
			logger.Info("Event self silenced")
			// alarm ended
			break
		}
		cancelNotification()
	}

	for {
		logger.Info("Checking for new calendar meetings")
		var event *calendar.Event
		var err error
		var timmeToWaitFor time.Time
		if *remindEnd {
			event, err = calendarClient.GetNextEndingEvent(context.Background())
		} else {
			event, err = calendarClient.GetNextUpcomingEvent(context.Background())
		}

		if err != nil {
			logger.Error("Failed to get meetings", err)
		} else if event != nil {
			if *remindEnd {
				timmeToWaitFor = event.EndTime
			} else {
				timmeToWaitFor = event.StartTime
			}

			waitDuration := time.Until(timmeToWaitFor) - *reminderTime

			logger.Infof("Found one [%s] waiting for %s", event.Title, waitDuration)
			if _, ok := notifiedEvents[event.ID]; !ok {
				logger.Infof("Waiting for event %s", event.Title)
				if waitDuration < 0 {
					notify(event)
				}

				// wait till 2 minutes before the event.
				// once triggerd do our control loop
				select {
				case <-time.After(waitDuration):
					notify(event)
					continue
				case <-time.After(emailPullInterval): //sleep
					continue
				}
			} else {
				logger.Infof("Event [%s] already processed, skipping", event.Title)
			}
		}

		// sleep a bit then check for new meetings
		logger.Info("None detected waiting...")
		time.Sleep(emailPullInterval)
	}
}
