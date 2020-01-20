#!/bin/sh
#
git stash -q --keep-index
make test
RESULT=$?
git stash pop -q
[ $RESULT -ne 0 ] && exit 1
exit 0