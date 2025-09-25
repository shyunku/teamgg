touch output.log
nohup ./teamgg > output.log 2>&1 &
tail -f output.log