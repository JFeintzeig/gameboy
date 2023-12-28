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
      bus.cartridge.write(address, value)
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
  //NoMBC is read-only
  return
}

type MBC1 struct {
  rawCartridgeData []Register8
  romSize uint32
  ramSize uint16

  // TODO: initialize romBank as 1
  romBank uint8
  ramBank uint8
  mode uint8
  isRAMEnabled bool

  //TODO: need to initialize this!
  ram []Register8
}

func (c *MBC1) read(address uint16) uint8 {
  var idx uint16
  switch {
  case address <= 0x3FFF:
    var idx uint16
    if c.mode == 0 {
      idx = address
    } else {
      var zerobanknumber uint8
      if c.romSize < 1024*1024 {
        zerobanknumber = 0
      } else if c.romSize == 1024*1024 {
        zerobanknumber = GetBit(c.ramBank,0) << 5
      } else if c.romSize == 2*1024*1024 {
        zerobanknumber = c.ramBank << 5
      } else {
        panic("unknown rom size for MBC1 reads")
      }
      idx = 0x4000 * uint16(zerobanknumber) + address
    }
    return c.rawCartridgeData[idx].read()
  case address >= 0x4000 && address <= 0x7FFF:
    // TODO
    highbanknumber := 0
    idx = 0x4000 * uint16(highbanknumber) + (address - 0x4000)
    return c.rawCartridgeData[idx].read()
  case address >= 0xA000 && address <= 0xBFFF:
    if c.isRAMEnabled {
      if c.ramSize == 2*1024 || c.ramSize == 8*1024 {
        idx = (address - 0xA000) % c.ramSize
      } else if c.ramSize == 32*1024 {
        // mode is either 0 or 1
        idx = uint16(c.mode) * 0x2000 * uint16(c.ramBank) + (address - 0xA000)
      }
      return c.ram[idx].read()
    } else {
      return 0xFF
    }
  default:
    panic("MBC1 unknown read operation")
  }
}

func (c *MBC1) write(address uint16, value uint8) {
  switch {
  case address <= 0x1FFF:
    if (address & 0x000F) == 0xA {
    c.isRAMEnabled = true
    } else {
      c.isRAMEnabled = false
    }
  case address >= 0x2000 && address <= 0x3FFF:
    var mask uint8
    if value == 0 {
      c.romBank = 1
      return
    } else if c.romSize >= 512*1024 {
      mask = 0b00011111
    } else if c.romSize == 256*1024 {
      mask = 0b00001111
    } else if c.romSize == 128*1024 {
      mask = 0b00000111
    } else if c.romSize == 64*1024 {
      mask = 0b00000011
    } else if c.romSize == 32 * 1024 {
      mask = 0b00000001
    }
    c.romBank = (mask & value)
  case address >= 0x4000 && address <= 0x5FFF:
    c.ramBank = (value & 0x03)
  case address >= 0x6000 && address <= 0x7FFF:
    c.mode = GetBit(value, 0)
  case address >= 0xA000 && address <= 0xBFFF:
    if c.isRAMEnabled {
      if c.ramSize == 2*1024 || c.ramSize == 8*1024 {
        idx := (address - 0xA000) % c.ramSize
        c.ram[idx].write(value)
      } else if c.ramSize == 32*1024 {
        // mode is either 0 or 1
        idx := uint16(c.mode) * 0x2000 * uint16(c.ramBank) + (address - 0xA000)
        c.ram[idx].write(value)
      } else {
        panic("unexpected ram size!")
      }
    }
  default:
    panic("unexpected MBC1 write operation")
  }
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

  cartridgeType := cartridgeData[0x147].read()
  var romSize uint32 = 32 * 1024 * (1 << cartridgeData[0x148].read())
  // TODO: finish calculating ramSize!
  var ramSize uint16 = uint16(cartridgeData[0x149].read())
  if cartridgeType == 0x00 {
    return &NoMBC{rawCartridgeData: cartridgeData}
  } else if cartridgeType == 0x01 {
    ramSize = 0x00
    return &MBC1{rawCartridgeData: cartridgeData, romSize: romSize, ramSize: ramSize, romBank: 1}
  } else if cartridgeType <= 0x03 {
    ram := make([]Register8, ramSize)
    return &MBC1{rawCartridgeData: cartridgeData, romSize: romSize, ramSize: ramSize, romBank: 1, ram: ram}
  } else {
    panic("unknown MBC type!")
  }
}
