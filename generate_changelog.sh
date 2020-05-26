#!/usr/bin/env bash

current_tag=$(git describe --tags)
previous_tag=$(git tag --sort=-creatordate   | head -n 2 | tail -n 1)

tag_date=$(git log -1 --pretty=format:'%ad' --date=short ${current_tag})
printf "## ${current_tag} (${tag_date})\n\n"
git log ${current_tag}...${previous_tag} --pretty=format:'*  %s [View](https://github.com/1xyz/coolbeans/commit/%H)' --reverse | grep -v Merge
