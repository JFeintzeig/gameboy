package cpu

import (
  "fmt"
  "io/ioutil"
  "log"
)

const CLOCK_SPEED uint64 = 4.19e6

type Motherboard struct {
  cpu Cpu
  memory [8192]byte
  vram [8192]byte
  ppu Ppu
  romFilePath *string
}

func (gb *Motherboard) LoadROM() {
  data, err := ioutil.ReadFile(*gb.romFilePath)
  if err != nil {
    log.Fatal("can't find file")
  }
  if len(data) != 32*1024 {
    log.Fatal("I can only do 32kb ROMs")
  }
  for index, element := range data {
    gb.memory[0x100 + index] = element
  }
}

func (gb *Motherboard) LoadBootROM() {
  data, err := ioutil.ReadFile("data/bootrom_dmg0.gb")
  if err != nil {
    log.Fatal("can't find boot rom")
  }
  for index, element := range data {
    gb.memory[index] = element
  }
}

func (gb *Motherboard) Execute() {
  for i := 0; i <= 100; i++ {
    fmt.Printf("%x: %x\n",gb.cpu.pc, gb.memory[gb.cpu.pc])
    gb.cpu.pc++
  }
}

type Cpu struct {
  regA uint8
  regF uint8
  regB uint8
  regC uint8
  regD uint8
  regE uint8
  regH uint8
  regL uint8
  pc uint16
  sp uint16
  clockSpeed uint64
}

func (cpu *Cpu) setFlagZ() {
}

func (cpu *Cpu) setFlagN() {
}

func (cpu *Cpu) setFlagH() {
}

func (cpu *Cpu) setFlagC() {
}

type Ppu struct {
  screen [160*144]uint8
}

func NewGameBoy(romFilePath *string) *Motherboard {
  cpu := Cpu{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, CLOCK_SPEED}
  ppu := Ppu{[160*144]uint8{}}
  gb := Motherboard{cpu, [8192]byte{}, [8192]byte{}, ppu, romFilePath}
  gb.LoadBootROM()
  return &gb
}
