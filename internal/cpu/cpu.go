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
  // TODO: execute micro-ops and make logic to choose when to advance
  // to next byte to decode...
  // TODO: timing
  for i := 0; i <= 10; i++ {
    //fmt.Printf("%x: %x, %b\n",gb.cpu.PC.read(), gb.memory[gb.cpu.PC.read()], gb.memory[gb.cpu.PC.read()])

    // TODO: tricky thing i need to figure out. this function
    // (a) decodes a new opcode
    // (b) adds its micro-instructions to the queue
    // (c) runs 1 micro instruction at a time for each clock cycle
    // (c) requires persisting (a). we only want to do (b) once for each
    // opcode, but the only way i know to persist (a) is to do it for
    // every (c). and then i _think_ gameboy timing can do a new (a) and (b)
    // while one (c) runs for the previous opcode, further complicating matters.
    // how to make this logic?
    // AND the first pass is hard -- i need oc but i dont want to increment
    // PC right away. maybe i can prefill ExecutionQueue with a dummy
    // instruction? janky!! maybe i can set currentOc as a value on cpu and
    // track state somehow? dont want it to be too complicated...

    fmt.Printf("%x %d\n", gb.cpu.PC.read(), gb.cpu.ExecutionQueue.Length())
    oc := ByteToOpcode(gb.memory[gb.cpu.PC.read()])
    fmt.Printf("Decoding %x X:%b Y:%b Z:%b P:%b Q:%b\n", oc.Full, oc.X, oc.Y, oc.Z, oc.P, oc.Q)

    fmt.Printf("%x %d\n", gb.cpu.PC.read(), gb.cpu.ExecutionQueue.Length())
    if gb.cpu.ExecutionQueue.Length() < 1 {
      // TODO: where to do this? i need PC pointing at
      // currently-running opcode if we are still executing
      // its microops, b/c microops depend on op
      fmt.Println("gonna increment PC")
      gb.cpu.PC.inc()
      oc = ByteToOpcode(gb.memory[gb.cpu.PC.read()])
      switch {
      case (oc.X == 0) && (oc.Z == 1) && (oc.Q == 0):
        fmt.Println("found it")
        inst := gb.cpu.InstructionMap["X0Z1Q0"]
        inst.AddOpsToQueue(&gb.cpu)
      default:
        fmt.Println("not implemented")
      }
    }

    // i want to run this whether i am adding ops to the queue or not
    // but we need _something_ in the queue to do this pop, otherwise
    // it fails at runtime
    // THIS IS BUGGY! LOOK AT THE PRINTS, I Step op before i should
    // and weird stuff is happening and i dont know why
    fmt.Printf("%x %d\n", gb.cpu.PC.read(), gb.cpu.ExecutionQueue.Length())
    if gb.cpu.ExecutionQueue.Length() > 0 {
      microop := gb.cpu.ExecutionQueue.Pop()
      fmt.Printf("Executing %x\n", oc.Full)
      (*microop)(&gb.cpu, &oc)
    }
  }
}

type Register8 struct {
  value uint8
  name string
}

func (reg *Register8) read() uint8 {
  return reg.value
}

func (reg *Register8) write(value uint8) {
  reg.value = value
}

func (reg *Register8) inc() {
  reg.value += 1
}

func (reg *Register8) dec() {
  reg.value -= 1
}

type Register16 struct {
  lo Register8
  hi Register8
}

func (reg *Register16) read() uint16 {
  return (uint16(reg.hi.value) << 8) | uint16(reg.lo.value)
}

func (reg *Register16) write(value uint16) {
  reg.writeLo(value)
  reg.writeHi(value)
}

func (reg *Register16) writeHi(value uint16) {
  reg.hi.write(uint8(value >> 8))
}

func (reg *Register16) writeLo(value uint16) {
  reg.lo.write(uint8(value&0xff))
}

func (reg *Register16) inc() {
  reg.write(reg.read()+1)
}

func (reg *Register16) dec() {
  reg.write(reg.read()-1)
}

func (reg *Register16) getName() string {
  return reg.hi.name + reg.lo.name
}

type Fifo[T any] struct {
  values []T
}

func (fifo *Fifo[T]) Push(val T) {
  fifo.values = append(fifo.values, val)
}

func (fifo *Fifo[T]) Pop() T {
  x, a := fifo.values[0], fifo.values[1:]
  fifo.values = a
  return x
}

func (fifo *Fifo[T]) Length() int {
  return len(fifo.values)
}

type Cpu struct {
  A Register8
  F Register8
  B Register8
  C Register8
  D Register8
  E Register8
  H Register8
  L Register8

  AF Register16
  BC Register16
  DE Register16
  HL Register16

  PC Register16
  SP Register16
  clockSpeed uint64

  rpTable []Register16

  InstructionMap map[string]Instruction

  ExecutionQueue Fifo[*func(*Cpu, *Opcode)]
}

func (cpu *Cpu) setFlagZ() {
}

func (cpu *Cpu) setFlagN() {
}

func (cpu *Cpu) setFlagH() {
}

func (cpu *Cpu) setFlagC() {
}

// TODO: implement ReadNN(), ReadN(), ReadD() on Cpu
func (cpu *Cpu) ReadNN() uint16 {
  return 0
}

type Ppu struct {
  screen [160*144]uint8
}

func NewGameBoy(romFilePath *string) *Motherboard {
  cpu := Cpu{}
 
  cpu.clockSpeed = CLOCK_SPEED
  cpu.rpTable = []Register16{cpu.BC, cpu.DE, cpu.HL, cpu.SP}

  cpu.InstructionMap = MakeInstructionMap()

  ppu := Ppu{[160*144]uint8{}}
  gb := Motherboard{cpu, [8192]byte{}, [8192]byte{}, ppu, romFilePath}
  gb.LoadBootROM()
  return &gb
}
