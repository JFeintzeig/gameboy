trap printout SIGINT
printout() {
    echo "DONE WITH TEST"
}

ROMDIR=$1

IFS=$'\n'
for file in `cat other_tests.txt`; do
  echo "***CPU INSTR TEST: `basename $file`"
  ~/projects/2023/gameboy/app -file $file -bootrom -fast
done
