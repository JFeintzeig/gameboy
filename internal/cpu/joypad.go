package cpu

import (
  "fmt"
)

type KeyPress struct {
  isPressed bool
  isJustPressed bool
}

type Joypad struct {
  bus Mediator

  value uint8
  keyboard map[string]KeyPress
}

func (j *Joypad) read() uint8 {
  var keypress uint8
  // select
  if GetBitBool(j.value, 5) {
    if p, ok := j.keyboard["start"]; p.isPressed && ok {
      SetBit(keypress, 3, 1)
    }
    if p, ok := j.keyboard["select"]; p.isPressed && ok {
      SetBit(keypress, 2, 1)
    }
    if p, ok := j.keyboard["a"]; p.isPressed && ok {
      SetBit(keypress, 1, 1)
    }
    if p, ok := j.keyboard["b"]; p.isPressed && ok {
      SetBit(keypress, 0, 1)
    }
  }
  // d-pad
  if GetBitBool(j.value, 4) {
    if p, ok := j.keyboard["down"]; p.isPressed && ok {
      SetBit(keypress, 3, 1)
    }
    if p, ok := j.keyboard["up"]; p.isPressed && ok {
      SetBit(keypress, 2, 1)
    }
    if p, ok := j.keyboard["left"]; p.isPressed && ok {
      SetBit(keypress, 1, 1)
    }
    if p, ok := j.keyboard["right"]; p.isPressed && ok {
      SetBit(keypress, 0, 1)
    }
  }
  fmt.Printf("joypad read: %08b\n", (j.value & 0xF0) | (keypress & 0x0F))
  return (j.value & 0xF0) | (keypress & 0x0F)
}

func (j *Joypad) write(val uint8) {
  // lower nibble read only
  j.value = (val & 0xF0) | (j.value & 0x0F)
}

func (j *Joypad) doCycle() {
  requestInterrupt := false
  for _, v := range j.keyboard {
    if v.isJustPressed {
      requestInterrupt = true
    }
  }

  if requestInterrupt {
    rIF := j.bus.ReadFromBus(IF)
    rIF = SetBitBool(rIF, 4, true)
    j.bus.WriteToBus(IF, rIF)
  }
}

func NewJoypad() *Joypad {
  keyboard := make(map[string]KeyPress)
  keyboard["up"] = KeyPress{false, false}
  keyboard["down"] = KeyPress{false, false}
  keyboard["left"] = KeyPress{false, false}
  keyboard["right"] = KeyPress{false, false}
  keyboard["a"] = KeyPress{false, false}
  keyboard["b"] = KeyPress{false, false}
  keyboard["start"] = KeyPress{false, false}
  keyboard["select"] = KeyPress{false, false}

  return &Joypad{value: 0xCF, keyboard: keyboard}
}
