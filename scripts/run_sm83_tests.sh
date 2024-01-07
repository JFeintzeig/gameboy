#!/bin/bash

# fyi had to rename cb \XX.json -> cb_XX.json before running this

for file in `ls ../../../gameboy_resources/jsmoo/misc/tests/GeneratedTests/sm83/v1/*.json`;
  do echo $file >> test.out
  printf %s `go test -file $file | wc -l` >> test.out
done
