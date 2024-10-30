#!/bin/bash -ue

curl -X POST -d '{"api":"startCommunication", "eventCode":1, "responseObject":{"category":0,"paramResult":1,"idm":"0011223344556677"}}' http://localhost:8889/pjf/api/eventTrigger
curl -X POST -d '{"api":"startCommunication", "eventCode":0, "responseObject":{"idm":"0011223344556677"}}' http://localhost:8889/pjf/api/eventTrigger
