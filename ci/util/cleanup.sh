#!/bin/bash
# ---
# Ryan Drew, 2022
# Delete all clusters starting with the prefix "circleci-cilium-perf-ci".
# Dangerous - make sure you are on the correct config.
# ---
list=`gcloud container clusters list --filter circleci`
echo "$list"
read -p "Are you sure you want to delete these? (y/N) >>> " confirmvar
if [ "$confirmvar" != "y" ]
then
    echo "Exiting"
    exit 0
fi
echo "$list" | tail -n +2 | cut -d' ' -f1 | xargs gcloud container clusters delete --quiet
