package main

import (
  "flag"
  "github.com/hajimehoshi/ebiten/v2"
  "jfeintzeig/gameboy/internal/cpu"
  "log"
)

const display = true

var (
  file *string
  bootrom *bool
  fast *bool
//  debug *bool
)

func init() {
  file = flag.String("file","data/Tetris.gb","path to file to load")
  bootrom = flag.Bool("bootrom",false,"set to true to use bootrom")
  fast = flag.Bool("fast",false,"set to true to make it faster than realtime")
}

func main() {
  flag.Parse()

  gb := cpu.NewGameBoy(file, *bootrom, *fast)

  if !display {
    gb.Execute()
  }

  ebiten.SetWindowSize(800, 720)
  ebiten.SetWindowTitle("Hello, World!")
  game, _ := cpu.NewEbitenGame(gb)

  // infinite loop at chip8.clockSpeed
  go gb.Execute()

  // display updates @ 60Hz via infinite loop in ebiten
  if err := ebiten.RunGame(game); err != nil {
    log.Fatal(err)
  }
}
