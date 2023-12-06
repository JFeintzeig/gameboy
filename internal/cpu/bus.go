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
)

type Mediator interface {
  ReadFromBus(uint16) uint8
  WriteToBus(uint16, uint8)
}

type Bus struct {
  memory [64*1024]Register8
  vram [8*1024]Register8
  ppu *Ppu
  timers *Timers
}

func (bus *Bus) ReadFromBus(address uint16) uint8 {
  switch {
    case address >= 0x8000 && address <= 0x9FFF:
      return bus.vram[address - 0x8000].read()
    case (address == DIV || address == TIMA || address == TMA || address == TAC):
      return bus.timers.read(address)
    case (address == LCDC || address == STAT || address == LY || address == LYC || address == SCX || address == SCY || address == WX || address == WY):
      return bus.ppu.read(address)
    default:
      return bus.memory[address].read()
  }
}

func (bus *Bus) WriteToBus(address uint16, value uint8) {
  switch {
    case address >= 0x8000 && address <= 0x9FFF:
      bus.vram[address - 0x8000].write(value)
    case (address == DIV || address == TIMA || address == TMA || address == TAC):
      bus.timers.write(address, value)
    case (address == LCDC || address == STAT || address == LY || address == LYC || address == SCX || address == SCY || address == WX || address == WY):
      bus.ppu.write(address, value)
    default:
      bus.memory[address].write(value)
  }
}

func (bus *Bus) LoadROM(romFilePath *string) {
  data, err := ioutil.ReadFile(*romFilePath)
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

func NewBus() *Bus {
  // need to initialize memory

  bus := Bus{}

  // returns a *Ppu
  ppu := NewPpu(&bus)
  bus.ppu = ppu

  timers := Timers{}
  timers.bus = &bus
  bus.timers = &timers

  return &bus
}
