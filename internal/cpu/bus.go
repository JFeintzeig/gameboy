package cpu

import (
  "io/ioutil"
  "log"
)

const (
  DIV = 0xFF04
  TIMA = 0xFF05
  TMA = 0xFF06
  TAC = 0xFF07
  LCDC = 0xFF40
  STAT = 0xFF41
  LY = 0xFF44
  LYC = 0xFF45
  SCY = 0xFF42
  SCX = 0xFF43
  WY = 0xFF4A
  WX = 0xFF4B
  OAM = 0xFE00
  BGP = 0xFF47
  OBP0 = 0xFF48
  OBP1 = 0xFF49
  BANK = 0xFF50
  P1 = 0xFF00
  IE = 0xFFFF
  IF = 0xFF0F
)

type Mediator interface {
  ReadFromBus(uint16) uint8
  WriteToBus(uint16, uint8)
}

type Bus struct {
  memory [64*1024]Register8
  ppu *Ppu
  timers *Timers
  joypad *Joypad
  romFilePath string
}

func (bus *Bus) ReadFromBus(address uint16) uint8 {
  switch {
    case address >= 0x8000 && address <= 0x9FFF:
      if bus.ppu.currentMode == M3 {
        return 0xFF
      } else {
        return bus.ppu.read(address)
      }
    case address >= 0xFE00 && address <= 0xFE9F:
      if bus.ppu.currentMode == M2 || bus.ppu.currentMode == M3 {
        return 0xFF
      } else {
        return bus.ppu.read(address)
      }
    case (address == DIV || address == TIMA || address == TMA || address == TAC):
      return bus.timers.read(address)
    case (address == LCDC || address == STAT || address == LY || address == LYC || address == SCX || address == SCY || address == WX || address == WY || address == BGP || address == OBP0 || address == OBP1):
      return bus.ppu.read(address)
    case address == P1:
      return bus.joypad.read()
    default:
      return bus.memory[address].read()
  }
}

func (bus *Bus) WriteToBus(address uint16, value uint8) {
  switch {
    case address >= 0x8000 && address <= 0x9FFF:
      if bus.ppu.currentMode == M3 {
        return
      } else {
        bus.ppu.write(address, value)
      }
    case address >= 0xFE00 && address <= 0xFE9F:
      if bus.ppu.currentMode == M2 || bus.ppu.currentMode == M3 {
        return
      } else {
        bus.ppu.write(address, value)
      }
    case (address == DIV || address == TIMA || address == TMA || address == TAC):
      bus.timers.write(address, value)
    case (address == LCDC || address == STAT || address == LY || address == LYC || address == SCX || address == SCY || address == WX || address == WY || address == BGP || address == OBP0 || address == OBP1):
      bus.ppu.write(address, value)
    case address == BANK:
      if bus.memory[address].read() == 0 {
        bus.LoadROM()
      }
      bus.memory[address].write(value)
    case address == P1:
      bus.joypad.write(value)
    default:
      bus.memory[address].write(value)
  }
}

func (bus *Bus) LoadROM() {
  data, err := ioutil.ReadFile(bus.romFilePath)
  if err != nil {
    log.Fatal("can't find file")
  }
  if len(data) > 32*1024 {
    log.Fatal("I can only do 32kb ROMs")
  }
  for address, element := range data {
    // starts at 0x0, first 256 bytes
    // will be overwitten by boot rom
    bus.WriteToBus(uint16(address), element)
  }
}

// TODO: somehow need to unmap this after the BootROM finishes
// monitor FF50 and then reload game cartridge?
// https://gbdev.io/pandocs/Memory_Map.html#io-ranges
func (bus *Bus) LoadBootROM() {
  data, err := ioutil.ReadFile("data/bootrom_dmg.gb")
  if err != nil {
    log.Fatal("can't find boot rom")
  }
  for address, element := range data {
    bus.WriteToBus(uint16(address), element)
  }
}

func NewBus(romFilePath string) *Bus {
  // need to initialize memory

  bus := Bus{}

  // returns a *Ppu
  ppu := NewPpu(&bus)
  bus.ppu = ppu

  timers := Timers{}
  timers.bus = &bus
  bus.timers = &timers
  bus.joypad = NewJoypad()
  bus.joypad.bus = &bus

  bus.romFilePath = romFilePath

  return &bus
}
