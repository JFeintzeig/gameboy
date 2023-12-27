package cpu

import (
  "sync"
)

type KeyPress struct {
  isJustReleased bool
  isJustPressed bool
}

type Joypad struct {
  bus Mediator

  value uint8
  mu sync.RWMutex
  keystate map[string]bool
  keyboard map[string]KeyPress
}

func (j *Joypad) read() uint8 {
  j.mu.RLock()
  var keypress uint8 = 0b1111
  // select
  if !GetBitBool(j.value, 5) {
    if p, ok := j.keystate["start"]; p && ok {
      keypress = SetBit(keypress, 3, 0)
    }
    if p, ok := j.keystate["select"]; p && ok {
      keypress = SetBit(keypress, 2, 0)
    }
    if p, ok := j.keystate["a"]; p && ok {
      keypress = SetBit(keypress, 1, 0)
    }
    if p, ok := j.keystate["b"]; p && ok {
      keypress = SetBit(keypress, 0, 0)
    }
  }
  // d-pad
  if !GetBitBool(j.value, 4) {
    if p, ok := j.keystate["down"]; p && ok {
      keypress = SetBit(keypress, 3, 0)
    }
    if p, ok := j.keystate["up"]; p && ok {
      keypress = SetBit(keypress, 2, 0)
    }
    if p, ok := j.keystate["left"]; p && ok {
      keypress = SetBit(keypress, 1, 0)
    }
    if p, ok := j.keystate["right"]; p && ok {
      keypress = SetBit(keypress, 0, 0)
    }
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
  for key, v := range j.keyboard {
    if s, ok := j.keystate[key]; v.isJustPressed && ok && !s {
      requestInterrupt = true
      j.keystate[key] = true
    }
    if s, ok := j.keystate[key]; v.isJustReleased && ok && s {
      j.keystate[key] = false
    }
    if v.isJustPressed && v.isJustReleased {
      panic("key pressed and released at same cycle\n")
    }
  }
  j.mu.RUnlock()

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

  keystate := make(map[string]bool)
  keystate["up"] = false
  keystate["down"] = false
  keystate["left"] = false
  keystate["right"] = false
  keystate["a"] = false
  keystate["b"] = false
  keystate["start"] = false
  keystate["select"] = false

  return &Joypad{value: 0xCF, keyboard: keyboard, keystate: keystate, mu: sync.RWMutex{}}
}
