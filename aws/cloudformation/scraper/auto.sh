#!/bin/bash
set -e

# make sure we have a clean checkout and the latest changes
git diff --exit-code || (echo "cannot run $0 with pending changes"; exit 1)
git pull --ff-only --quiet

# re-run the code generator
[ ! -d .scraper-cache ] || rm -rf .scraper-cache
go generate .

# if nothing changed, we're done
if git diff --exit-code >/dev/null ; then
	exit 0
fi

# abort if we broke something
go test ./...

# commit and apply the changes
git add -A
git commit -m "schema: apply changes from AWS documentation updates (auto commit)"
git push
