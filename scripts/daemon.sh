touch output.log
nohup ./teamgg > output.log 2>&1 &
echo "teamgg daemon started."
tail -f output.log