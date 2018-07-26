#!/bin/bash
cfg=dashboard.xml
dashboard_pid=`ps ux | grep "start $cfg" | grep -v grep | awk '{print $2}'`
if [ -n "$dashboard_pid" ]; then
    kill $dashboard_pid
    while [ -d /proc/$dashboard_pid ]
    do
        echo "waiting dashboard stop"
        sleep 1
    done
    echo "dashboard is stoped"
fi
echo "dashboard is ready start"
nohup ./dashboard start $cfg > dashboard.log 2>&1 &
echo "dashboard is running"
