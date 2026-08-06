count=$(pkill -e -f 'teamgg' 2>/dev/null | wc -l)
echo "SIGKILL sent to $count processes."