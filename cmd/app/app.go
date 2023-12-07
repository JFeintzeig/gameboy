package main

import (
  "flag"
  "github.com/hajimehoshi/ebiten/v2"
  "jfeintzeig/gameboy/internal/cpu"
  "log"
)

const display = false

var (
  file *string
//  debug *bool
)

func init() {
  file = flag.String("file","data/Tetris.gb","path to file to load")
}

func main() {
  flag.Parse()

  gb := cpu.NewGameBoy(file, false)

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
