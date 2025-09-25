pkill -f teamgg.sh
touch output.log
chmod +x build_run_teamgg.sh
nohup ./build_run_teamgg.sh > output.log 2>&1 &
tail -f output.log