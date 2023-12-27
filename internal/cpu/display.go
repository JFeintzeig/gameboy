package cpu

import (
  "image/color"
  "github.com/hajimehoshi/ebiten/v2"
  "github.com/hajimehoshi/ebiten/v2/inpututil"
)

var (
  pixel0 = ebiten.NewImage(5,5)
  pixel1 = ebiten.NewImage(5,5)
  pixel2 = ebiten.NewImage(5,5)
  pixel3 = ebiten.NewImage(5,5)
)

func init() {
  // reversed colors
  pixel0.Fill(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
  pixel1.Fill(color.RGBA{0xAA, 0xAA, 0xAA, 0xFF})
  pixel2.Fill(color.RGBA{0x55, 0x55, 0x55, 0xFF})
  pixel3.Fill(color.RGBA{0x00, 0x00, 0x00, 0xFF})
}

type Game struct {
  cpu *Cpu
  keyboard map[string]ebiten.Key
}

func (g *Game) Update() error {
  for keyName, key := range g.keyboard {
    if keypress, ok := g.cpu.Bus.joypad.keyboard[keyName]; ok {
      keypress.isJustReleased = inpututil.IsKeyJustReleased(key)
      keypress.isJustPressed = inpututil.IsKeyJustPressed(key)
      g.cpu.Bus.joypad.mu.Lock()
      g.cpu.Bus.joypad.keyboard[keyName] = keypress
      g.cpu.Bus.joypad.mu.Unlock()
    }
  }
  return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
  for index, element := range g.cpu.Bus.ppu.screen {
    op := &ebiten.DrawImageOptions{}
    y := int(index / 160)
    x := int(index % 160)
    op.GeoM.Translate(float64(x*5),float64(y*5))
    switch element {
    case 0x0:
      screen.DrawImage(pixel0, op)
    case 0x1:
      screen.DrawImage(pixel1, op)
    case 0x2:
      screen.DrawImage(pixel2, op)
    case 0x3:
      screen.DrawImage(pixel3, op)
    }
  }
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
  // TODO: abstract this away.
  return 800, 720
}

func NewEbitenGame(cpu *Cpu) (*Game, error) {
  keyboard := make(map[string]ebiten.Key)
  keyboard["up"] = ebiten.KeyW
  keyboard["down"] = ebiten.KeyS
  keyboard["left"] = ebiten.KeyA
  keyboard["right"] = ebiten.KeyD
  keyboard["a"] = ebiten.KeyK
  keyboard["b"] = ebiten.KeyJ
  keyboard["start"] = ebiten.KeyI
  keyboard["select"] = ebiten.KeyU

  g := &Game{
    cpu,
    keyboard,
  }
  return g, nil
}
