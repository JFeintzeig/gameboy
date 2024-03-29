app:
	go build cmd/app/app.go

test_instrs: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/gb-test-roms/cpu_instrs/individual/

test_timer: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/mts-20221022-1430-8d742b9/acceptance/timer/

test_ppu: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/mts-20221022-1430-8d742b9/acceptance/ppu/

test_instr_timing: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/gb-test-roms/instr_timing/

test_mem_timing: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/gb-test-roms/mem_timing/individual/

test_mem_timing2: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/gb-test-roms/mem_timing-2/rom_singles/

test_other: app
	./scripts/run_other_tests.sh

test_acid: app
	./app -file ../gameboy_resources/dmg-acid2/dmg-acid2.gb -bootrom -fast

test_mbc1: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/mts-20221022-1430-8d742b9/emulator-only/mbc1/
