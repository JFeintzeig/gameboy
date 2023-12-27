package cpu

import (
  "fmt"
	"log"
  "os"
)

const BOOT_ROM_FILEPATH = "data/bootrom_dmg.gb"

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
  OAM_START = 0xFE00
  OAM_END = 0xFE9F
  BGP = 0xFF47
  OBP0 = 0xFF48
  OBP1 = 0xFF49
  BANK = 0xFF50
  P1 = 0xFF00
  IE = 0xFFFF
  IF = 0xFF0F
  DMA = 0xFF46
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
  cartridge Cartridge
  bootROM Cartridge
  isBootROMMapped bool

  // DMA
  dmaInProgress bool
  dmaStartAddress uint16
  dmaCounter uint8
}

func (bus *Bus) ReadFromBus(address uint16) uint8 {
  switch {
    case address < 0x100:
      if bus.isBootROMMapped {
        return bus.bootROM.read(address)
      } else {
        return bus.cartridge.read(address)
      }
    case address >= 0x100 && address < 0x8000:
      return bus.cartridge.read(address)
    case address >= 0x8000 && address <= 0x9FFF:
      mode := Mode(bus.ReadFromBus(STAT) & 0x03)
      if mode == M3 {
        return 0xFF
      } else {
        return bus.ppu.read(address)
      }
    case address >= OAM_START && address <= OAM_END:
      mode := Mode(bus.ReadFromBus(STAT) & 0x03)
      if mode == M2 || mode == M3 {
        return 0xFF
      } else {
        return bus.ppu.read(address)
      }
    case (address == DIV || address == TIMA || address == TMA || address == TAC):
      fmt.Printf("read from timers\n")
      return bus.timers.read(address)
    case (address == LCDC || address == STAT || address == LY || address == LYC || address == SCX || address == SCY || address == WX || address == WY || address == BGP || address == OBP0 || address == OBP1):
      return bus.ppu.read(address)
    case address == P1:
      return bus.joypad.read()
    case address == DMA:
      return uint8(bus.dmaStartAddress >> 8)
    default:
      return bus.memory[address].read()
  }
}

func (bus *Bus) WriteToBus(address uint16, value uint8) {
  switch {
    case address < 0x8000:
      // ROM is ready-only
      return
    case address >= 0x8000 && address <= 0x9FFF:
      mode := Mode(bus.ReadFromBus(STAT) & 0x03)
      if mode == M3 {
        return
      } else {
        bus.ppu.write(address, value)
      }
    case address >= OAM_START && address <= OAM_END:
      mode := Mode(bus.ReadFromBus(STAT) & 0x03)
      if mode == M2 || mode == M3 {
        return
      } else {
        bus.ppu.write(address, value)
      }
    case (address == DIV || address == TIMA || address == TMA || address == TAC):
      fmt.Printf("write to timers %04X %02X\n", address, value)
      bus.timers.write(address, value)
    case (address == LCDC || address == STAT || address == LY || address == LYC || address == SCX || address == SCY || address == WX || address == WY || address == BGP || address == OBP0 || address == OBP1):
      bus.ppu.write(address, value)
    case address == BANK:
      if bus.memory[address].read() == 0 {
        bus.isBootROMMapped = false
      }
      bus.memory[address].write(value)
    case address == P1:
      bus.joypad.write(value)
    case address == DMA:
      bus.dmaCounter = 0
      bus.dmaStartAddress = uint16(value) << 8
      bus.dmaInProgress = true
    default:
      bus.memory[address].write(value)
  }
}

func (bus *Bus) doCycle() {
  if !bus.dmaInProgress {
    return
  }
  if bus.dmaCounter == 0 {
    bus.dmaCounter += 1
    // do nothing for one cycle
    return
  } else if bus.dmaCounter > 160 {
    bus.dmaInProgress = false
    bus.dmaCounter = 0
    return
  } else {
    // directly access memory, don't go through WriteToBus // ReadFromBus because of blocking?
    sourceAddress := bus.dmaStartAddress + uint16(bus.dmaCounter) - 1
    destAddress := OAM_START + uint16(bus.dmaCounter) - 1
    // TODO: need to read directly so i can block reads
    value := bus.ReadFromBus(sourceAddress)
    bus.ppu.write(destAddress, value)
    bus.dmaCounter += 1
  }
}

func NewBus(romFilePath string, useBootROM bool) *Bus {
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
  bus.cartridge = NewCartridge(romFilePath)
  bus.bootROM = NewCartridge(BOOT_ROM_FILEPATH)
  bus.isBootROMMapped = useBootROM

  bus.dmaInProgress = false

  return &bus
}

type Cartridge interface {
  read(uint16) uint8
  write(uint16, uint8)
}


type NoMBC struct {
  rawCartridgeData []Register8
}

func (c *NoMBC) read(address uint16) uint8 {
  return c.rawCartridgeData[address].read()
}

func (c *NoMBC) write(address uint16, value uint8) {
  c.rawCartridgeData[address].write(value)
}

func NewCartridge(romFilePath string) Cartridge {
  fi, err := os.Stat(romFilePath)
  fSize := fi.Size()
  data, err := os.ReadFile(romFilePath)
  if err != nil {
    log.Fatal("can't find file")
  }
  if len(data) > 32*1024 {
    log.Fatal("I can only do 32kb ROMs")
  }
  cartridgeData := make([]Register8, fSize)
  // implicit: index of array is address in memory :eek:
  for address, element := range data {
    cartridgeData[address].write(element)
  }
  return &NoMBC{rawCartridgeData: cartridgeData}
}
