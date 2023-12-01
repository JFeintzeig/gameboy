trap printout SIGINT
printout() {
    echo "DONE WITH TEST"
}

ROMDIR=$1

cd ~/projects/2023/gameboy_resources/gameboy-doctor
IFS=$'\n'
for file in `ls ${ROMDIR}/*.gb`; do
  echo "***CPU INSTR TEST: `basename $file`"
  ~/projects/2023/gameboy/app -file $file
done

cd -
