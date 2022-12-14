package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Event struct {
	ID        string
	StartTime time.Time
	EndTime   time.Time
	Title     string
	Status    string
	Location  string
}

type Calendar struct {
	srv           *calendar.Service
	calendarID    string
	calendarQuery string
	logger        golog.Logger
}

func NewClient(ctx context.Context, jsonCredsFile, calendarID string, calendarQuery string, logger golog.Logger) (*Calendar, error) {
	b, err := os.ReadFile(jsonCredsFile)
	if err != nil {
		return nil, err
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to parse client secret file to config")
	}

	client, err := getClient(config)
	if err != nil {
		return nil, err
	}
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &Calendar{
		srv:        srv,
		calendarID: calendarID,
		logger:     logger.Desugar().Named("calendar").Sugar(),
	}, nil
}

func (c *Calendar) GetNextUpcomingEvent(ctx context.Context) (*Event, error) {
	c.logger.Debug("GetNextUpcomingEvent")
	req := c.buildReqest().OrderBy("startTime")
	events, err := req.Do()

	if err != nil {
		return nil, err
	}

	for _, e := range events.Items {

		event, err := formatEvent(e)
		if err != nil {
			return nil, err
		}

		if event.StartTime.Before(time.Now()) {
			c.logger.Debugf(" - GetNextUpcomingEvent [%s] %s skipping, start time before now", event.Title, event.StartTime)
			continue
		}

		return event, nil
	}

	return nil, nil
}

func (c *Calendar) GetNextEndingEvent(ctx context.Context) (*Event, error) {
	c.logger.Info("GetNextEndingEvent")
	req := c.buildReqest().OrderBy("startTime")
	events, err := req.Do()

	if err != nil {
		return nil, err
	}

	for _, e := range events.Items {
		event, err := formatEvent(e)
		if err != nil {
			return nil, err
		}

		if event.EndTime.Before(time.Now()) {
			c.logger.Debugf(" - GetNextEndingEvent [%s] %s skipping, end time before now", event.Title, event.EndTime)
			continue
		}

		return event, nil
	}

	return nil, nil
}

func formatEvent(e *calendar.Event) (*Event, error) {
	sTime, err := time.Parse(time.RFC3339, e.Start.DateTime)
	if err != nil {
		return nil, err
	}
	eTime, err := time.Parse(time.RFC3339, e.End.DateTime)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:        e.Id,
		StartTime: sTime,
		EndTime:   eTime,
		Title:     e.Summary,
		Status:    e.Status,
		Location:  e.Location,
	}, nil
}

func (c *Calendar) buildReqest() *calendar.EventsListCall {
	tNow := time.Now()
	tEnd := tNow.Add(time.Minute * 60)

	req := c.srv.Events.List(c.calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(tNow.Format(time.RFC3339)).
		TimeMax(tEnd.Format(time.RFC3339))

	if c.calendarQuery != "" {
		req = req.Q(c.calendarQuery)
	}

	return req
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) (*http.Client, error) {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokenFile := "token.json"
	token, err := tokenFromFile(tokenFile)
	if err != nil {
		token, err := getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		saveToken(tokenFile, token)
	}
	return config.Client(context.Background(), token), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, errors.Wrap(err, "unable to read authorization code")
	}

	token, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, errors.Wrap(err, "unable to retrieve token from web")
	}

	return token, nil
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrap(err, "Unable to cache oauth token")
	}

	defer f.Close()
	json.NewEncoder(f).Encode(token)

	return nil
}
