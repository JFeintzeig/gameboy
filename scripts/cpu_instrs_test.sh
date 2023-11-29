trap printout SIGINT
printout() {
    echo "DONE WITH TEST"
}

cd ~/projects/2023/gameboy_resources/gameboy-doctor
IFS=$'\n'
for file in `ls ~/projects/2023/gameboy_resources/gb-test-roms/cpu_instrs/individual/*.gb`; do
  echo "***CPU INSTR TEST: `basename $file`"
  ~/projects/2023/gameboy/app -file $file
done

cd -
