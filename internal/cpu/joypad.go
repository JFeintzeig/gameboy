package cpu

import (
  "fmt"
  "sync"
)

type KeyPress struct {
  isPressed bool
  isJustPressed bool
}

type Joypad struct {
  bus Mediator

  value uint8
  mu sync.RWMutex
  keyboard map[string]KeyPress
}

func (j *Joypad) read() uint8 {
  j.mu.RLock()
  var keypress uint8 = 0b1111
  // select
  if GetBitBool(j.value, 5) {
    if p, ok := j.keyboard["start"]; p.isPressed && ok {
      SetBit(keypress, 3, 0)
    }
    if p, ok := j.keyboard["select"]; p.isPressed && ok {
      SetBit(keypress, 2, 0)
    }
    if p, ok := j.keyboard["a"]; p.isPressed && ok {
      SetBit(keypress, 1, 0)
    }
    if p, ok := j.keyboard["b"]; p.isPressed && ok {
      SetBit(keypress, 0, 0)
    }
  }
  // d-pad
  if GetBitBool(j.value, 4) {
    if p, ok := j.keyboard["down"]; p.isPressed && ok {
      SetBit(keypress, 3, 0)
    }
    if p, ok := j.keyboard["up"]; p.isPressed && ok {
      SetBit(keypress, 2, 0)
    }
    if p, ok := j.keyboard["left"]; p.isPressed && ok {
      SetBit(keypress, 1, 0)
    }
    if p, ok := j.keyboard["right"]; p.isPressed && ok {
      SetBit(keypress, 0, 0)
    }
  }
  if (keypress & 0x0F) != 0b1111 {
    // TODO: this never shows a key as being pressed :(
    fmt.Printf("reading a joypad keypress!!!!: %08b\n", (j.value & 0xF0) | (keypress & 0x0F))
  }
  j.mu.RUnlock()
  return (j.value & 0xF0) | (keypress & 0x0F)
}

func (j *Joypad) write(val uint8) {
  // lower nibble read only
  j.value = (val & 0xF0) | (j.value & 0x0F)
}

func (j *Joypad) doCycle() {
  requestInterrupt := false
  j.mu.RLock()
  for _, v := range j.keyboard {
    // TODO: this triggers many interrupts b/c display.go runs at 60Hz vs this runs way faster
    // Maybe easier to refactor to channels
    if v.isJustPressed {
      requestInterrupt = true
    }
  }
  j.mu.RUnlock()

  if requestInterrupt {
    fmt.Printf("sending joypad interrupt...\n")
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

  return &Joypad{value: 0xCF, keyboard: keyboard, mu: sync.RWMutex{}}
}
