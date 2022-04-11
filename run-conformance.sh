#!/bin/bash

set -o pipefail

i=1
while true 
do 
  echo "########TEST Run $i###########"
  if (make test-conformance | tee test.out) then 
    kubectl get ns | grep "test-" | awk '{print $1}' | xargs -L1 kubectl delete ns
    rm test.out
  else
    echo "Failed after: $i"
    break
  fi
  i=$((i+1))
done
