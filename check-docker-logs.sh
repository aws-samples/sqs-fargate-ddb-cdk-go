#!/bin/bash
for (( ; ; ))
do
  echo "Printing Docker logs"
  echo $(docker logs $1)
  echo "Press Ctrl+C to stop..."
done
