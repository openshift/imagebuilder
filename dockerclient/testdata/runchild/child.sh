#!/bin/sh
echo "starting child"
( sleep 10; echo "child done" ) &
echo "script done"