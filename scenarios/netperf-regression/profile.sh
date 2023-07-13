#!/bin/sh

echo "Starting profile..."

bpftrace /configs/profile.bt > /store/profile.txt

echo "Done"
