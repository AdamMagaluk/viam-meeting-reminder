package device

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"golang.org/x/net/context"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/utils"
	"go.viam.com/utils/rpc"
)

const (
	ledPinName             = "8"
	buzzerPinName          = "10"
	buttonInterruptPinName = "12"
	boardName              = "board"
	buttonInterruptName    = "button"

	notificationDwell   = 250 * time.Millisecond
	interruptCheckDwell = 25 * time.Millisecond
)

type ReminderDevice struct {
	ledPin    board.GPIOPin
	buzzerPin board.GPIOPin
	button    board.DigitalInterrupt
	client    *client.RobotClient
	logger    golog.Logger
}

type credentials struct {
	Robot  string `json:"robot"`
	Secret string `json:"secret"`
}

func (r *ReminderDevice) Close(ctx context.Context) error {
	return r.client.Close(ctx)
}

func (r *ReminderDevice) StartNotification(ctx context.Context) {
	go func() {
		defer r.setLedAndBuzzerOn(context.Background(), false)

		nextState := true
		for {
			if ctx.Err() != nil {
				return
			}

			r.setLedAndBuzzerOn(ctx, nextState)

			select {
			case <-ctx.Done(): //context cancelled
				return
			case <-time.After(notificationDwell): //sleep
				nextState = !nextState
				continue
			}
		}
	}()
}

func (r *ReminderDevice) setLedAndBuzzerOn(ctx context.Context, high bool) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		r.logger.Infof("Turning LED %s", high)
		if err := r.ledPin.Set(ctx, high, nil); err != nil {
			r.logger.Errorf("failed to set led: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		r.logger.Infof("Turning Buzzer %s", high)
		if err := r.buzzerPin.Set(ctx, high, nil); err != nil {
			r.logger.Errorf("failed to set buzzer: %v", err)
		}
	}()

	wg.Wait()
}

func (r *ReminderDevice) NotifyOnceIfButtonPress(ctx context.Context) chan bool {
	callback := make(chan bool, 1)
	go func() {
		defer close(callback)

		receivedInitialValue := false
		lastValue := int64(0)
		for {
			if ctx.Err() != nil {
				return
			}

			val, err := r.button.Value(ctx, nil)
			if err != nil {
				r.logger.Errorf("failed to get button value %v", err)
				continue
			} else if !receivedInitialValue {
				receivedInitialValue = true
				lastValue = val
				continue
			}

			if val > lastValue+5 {
				// detected change
				callback <- true
				return
			}

			lastValue = val

			select {
			case <-ctx.Done(): //context cancelled
				return
			case <-time.After(interruptCheckDwell): //sleep
				continue
			}
		}
	}()

	return callback
}

func NewDevice(credFile string) (*ReminderDevice, error) {
	creds, err := loadCredentialsFromFile(credFile)
	if err != nil {
		return nil, err
	}

	logger := golog.NewDevelopmentLogger("device")
	robot, err := client.New(
		context.Background(),
		creds.Robot,
		logger,
		client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
			Type:    utils.CredentialsTypeRobotLocationSecret,
			Payload: creds.Secret,
		})),
	)
	if err != nil {
		return nil, err
	}

	board, err := board.FromRobot(robot, boardName)
	if err != nil {
		return nil, err
	}

	ledPin, err := board.GPIOPinByName(ledPinName)
	if err != nil {
		return nil, err
	}

	buzzerPin, err := board.GPIOPinByName(buzzerPinName)
	if err != nil {
		return nil, err
	}

	buttonInterrupt, ok := board.DigitalInterruptByName(buttonInterruptName)
	if !ok {
		return nil, err
	}

	reminderDevice := &ReminderDevice{
		ledPin:    ledPin,
		buzzerPin: buzzerPin,
		button:    buttonInterrupt,
		client:    robot,
		logger:    logger,
	}

	// reset
	reminderDevice.setLedAndBuzzerOn(context.Background(), false)

	return reminderDevice, nil
}

func loadCredentialsFromFile(credFile string) (*credentials, error) {
	b, err := os.ReadFile(credFile)
	if err != nil {
		return nil, err
	}

	var creds credentials
	err = json.Unmarshal(b, &creds)
	if err != nil {
		return nil, err
	}

	return &creds, nil
}
