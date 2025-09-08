touch output.log
nohup ./build_run.sh > output.log 2>&1 &
tail -f output.log