package main

import (
	"context"
	"time"

	"adammagaluk.io/meeting-reminder/calendar"
	"adammagaluk.io/meeting-reminder/device"

	"github.com/edaniels/golog"
)

const (
	emailPullInterval    = 5 * time.Second
	reminderBefore       = 2 * time.Minute
	notificationDuration = 30 * time.Second
)

func main() {
	logger := golog.NewDevelopmentLogger("client")

	notifiedEvents := make(map[string]bool)

	reminderDevice, err := device.NewDevice("./robot-config.json")
	if err != nil {
		logger.Fatal(err)
	}
	defer reminderDevice.Close(context.Background())

	calendarClient, err := calendar.NewClient(context.Background(), "./calendar_oauth_creds.json")
	if err != nil {
		logger.Fatal(err)
	}

	notify := func(event *calendar.Event) {
		notifiedEvents[event.ID] = true
		logger.Info("Notifying for event %s", event.Title)

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
		event, err := calendarClient.GetNextUpcomingEvent(context.Background())
		if err != nil {
			logger.Errorf("Failed to get meetings")
		} else if event != nil {

			logger.Info("Found one waiting for %s", time.Until(event.StartTime))
			if _, ok := notifiedEvents[event.ID]; !ok {
				logger.Info("Skipping for event %s", event.Title)

				// wait till 2 minutes before the event.
				// once triggerd do our control loop
				select {
				case <-time.After(time.Until(event.StartTime) - reminderBefore):
					notify(event)
					continue
				case <-time.After(emailPullInterval): //sleep
					continue
				}
			}
		}

		// sleep a bit then check for new meetings
		logger.Info("None detected waiting...")
		time.Sleep(emailPullInterval)
	}
}
