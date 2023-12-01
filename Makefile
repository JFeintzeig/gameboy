app:
	go build cmd/app/app.go

test_instrs: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/gb-test-roms/cpu_instrs/individual/

test_timer: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/mts-20221022-1430-8d742b9/acceptance/timer/

test_ppu: app
	./scripts/run_test_roms.sh ~/projects/2023/gameboy_resources/mts-20221022-1430-8d742b9/acceptance/ppu/
