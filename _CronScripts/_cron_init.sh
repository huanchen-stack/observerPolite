#!/bin/bash

output=$(python3 _cron_init.py)
read -r dbCollection dbCollectionComp <<< "$output"

go build -o observer
current_date=$(date +"%Y%m%d")
nohup ./observer -dbCollection "$dbCollection" -dbCollectionComp "$dbCollectionComp" > "${current_date}.out" 2>&1 &

echo "Script executed. Observer is running in the background with flags: $dbCollection, $dbCollectionComp."
