#!/bin/bash -ue

curl -X POST -d '{"api":"startEventListen", "eventCode":0}' http://localhost:8889/pjf/api/eventTrigger

