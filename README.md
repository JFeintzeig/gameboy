# Gameboy
A gameboy emulator in Go. Current status:
- Tetris works! 🥳
- Timing accuracy: Timers + CPU instruction timing + memory timing tests all succeed, a few of the PPU timing tests work.
- PPU accuracy: acid2 test passes 🙂
- Other games don't run yet! There's some tricky bugs in my interrupt servicing routine that I need to figure out.
- MBC1 implemented but still buggy.

# Setup
- To run the tests in the Makefile: the tests assume you have a sibling directory named `gameboy_resources`, into which you've checked out [gameboy-doctor](https://github.com/robert/gameboy-doctor) and [gb-test-roms](https://github.com/retrio/gb-test-roms) in the parent directory, so your directory structure should look like:
  - gameboy/ (this repo)
      - internal/
      - ...
  - gameboy_resources/
      - gameboy-doctor/
      - gb-test-roms/
