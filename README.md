# Viam Meeting Reminder

A sample device that connects a LED, Buzzer, Button as a alarm clock for Google Calendar
meetings 2 minutes before they start all powered by [Viam](https://viam.com).

![Meeting Reminder Image](/docs/images/meeting-reminder.jpg)

`git clone https://github.com/AdamMagaluk/viam-meeting-reminder.git`

Sparkfun Wishlist: https://www.sparkfun.com/wish_lists/169342

Note: Not supported by Viam officially.

### 1. Create a Oauth2 Client Google Cloud

Follow the [Google documentation here](https://developers.google.com/calendar/api/quickstart/go)

Download the credentials file and save to `./calendar_oauth_creds.json`

### 2. Setup Pi and Create Viam Robot

Follow the steps [here](https://docs.viam.com/docs/getting-started/installation/).

### 3. Configue the robot

Create a new board component called `board` with model as `pi`.

Add the following attributes section:

```json
{
  "digital_interrupts": [
    {
      "name": "button",
      "pin": "12"
    }
  ]
}
```

Add the following Proccesses Section:

```json
[
  {
    "cwd": "/home/adam",
    "id": "calendar-reminder",
    "log": true,
    "name": "./calendar_reminder"
  }
]
```

### 4. Configure the local robot secret

Create a json file called `./robot-config.json`:

```json
{
    "robot": "<robot-host>",
    "secret": "<robot-location-secret>"
}
```

Fill in the details that are shown in the connect page. See `address` and `secret`.

### 5. Deploy Code to Pi

Run the code locally to get it setup. Follow direction in prompt to get Google Calendar auth token.

Once working deploy to the pi. `./deploy-to-pi.sh`