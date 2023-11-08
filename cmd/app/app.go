package main

import (
  "flag"
  "fmt"

  "jfeintzeig/gameboy/internal/cpu"
)

var (
  file *string
//  debug *bool
)

func init() {
  // file = flag.String("file","../gameboy_resources/gb-bootroms/bin/dmg.bin","path to file to load")
  file = flag.String("file","data/Tetris.gb","path to file to load")
}

func main() {
  flag.Parse()

  fmt.Println("Starting up...")
  gb := cpu.NewGameBoy(file)

  // ebiten.SetWindowSize(640, 320)
  // ebiten.SetWindowTitle("Hello, World!")
  // game, _ := display.NewGame(chip8)

  // infinite loop at chip8.clockSpeed
  gb.Execute()

  // display updates @ 60Hz via infinite loop in ebiten
  // if err := ebiten.RunGame(game); err != nil {
  //   log.Fatal(err)
  // }
}
