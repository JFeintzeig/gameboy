package cpu

const CLOCK_SPEED uint64 = 4.19e6

type Motherboard struct {
  cpu Cpu
  memory [8192]byte
  vram [8192]byte
  ppu Ppu
}

func (gb *Motherboard) LoadFile(filePath string) {
}

func (gb *Motherboard) Execute() {
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

func NewGameBoy() *Motherboard {
  cpu := Cpu{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, CLOCK_SPEED}
  ppu := Ppu{[160*144]uint8{}}
  gb := Motherboard{cpu, [8192]byte{}, [8192]byte{}, ppu}
  return &gb
}
