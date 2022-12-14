#!/bin/bash

env GOOS=linux GOARCH=arm64 go build -o calendar_reminder

scp calendar_reminder robot-config.json token.json calendar_oauth_creds.json bot@meeting-bot.local:/home/bot/