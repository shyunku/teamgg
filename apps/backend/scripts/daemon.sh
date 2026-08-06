touch output.log
setsid nohup ./teamgg > output.log 2>&1 < /dev/null &
echo $! > teamgg.pid
echo "teamgg daemon started. pid=$(cat teamgg.pid)"
tail -f output.log