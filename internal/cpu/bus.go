package cpu

import (
  "io/ioutil"
  "log"
)

type Mediator interface {
  ReadFromBus(uint16) uint8
  WriteToBus(uint16, uint8)
}

type Bus struct {
  memory [64*1024]Register8
  ppu *Ppu
  timers *Timers
}

func (bus *Bus) ReadFromBus(address uint16) uint8 {
  if address >= 0x8000 && address <= 0x9FFF {
    return bus.ppu.readVRAM(address)
  } else if address >= 0xFF04 && address <= 0xFF07 {
    return bus.timers.read(address)
  } else if address >= 0xFF40 && address <= 0xFF4B {
    return bus.ppu.readRegister(address)
  } else {
    return bus.memory[address].read()
  }
}

func (bus *Bus) WriteToBus(address uint16, value uint8) {
  if address >= 0x8000 && address <= 0x9FFF {
    bus.ppu.writeVRAM(address, value)
  } else if address >= 0xFF04 && address <= 0xFF07 {
    bus.timers.write(address, value)
  } else if address >= 0xFF40 && address <= 0xFF4B {
    bus.ppu.writeRegister(address, value)
  } else {
    bus.memory[address].write(value)
  }
}

func (bus *Bus) LoadROM(romFilePath *string) {
  data, err := ioutil.ReadFile(*romFilePath)
  if err != nil {
    log.Fatal("can't find file")
  }
  if len(data) != 32*1024 {
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
