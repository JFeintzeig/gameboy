package cpu

import (
  "fmt"
  "io/ioutil"
  "log"
)

const CLOCK_SPEED uint64 = 4.19e6

type Bus struct {
  // TODO: implement memory mapper
  // for now i just make memory a big array for 16 bit address space
  memory [64*1024]byte
  vram [8192]byte
  ppu Ppu
}

func (gb *Bus) LoadROM(romFilePath *string) {
  data, err := ioutil.ReadFile(*romFilePath)
  if err != nil {
    log.Fatal("can't find file")
  }
  if len(data) != 32*1024 {
    log.Fatal("I can only do 32kb ROMs")
  }
  for index, element := range data {
    // starts at 0x0, first 256 bytes
    // will be overwitten by boot rom
    gb.memory[index] = element
  }
}

func (gb *Bus) LoadBootROM() {
  data, err := ioutil.ReadFile("data/bootrom_dmg0.gb")
  if err != nil {
    log.Fatal("can't find boot rom")
  }
  for index, element := range data {
    gb.memory[index] = element
  }
}

func (cpu *Cpu) OpcodeToInstruction(op Opcode) *Instruction {
      switch {
      case (op.X == 0) && (op.Z == 1) && (op.Q == 0):
        inst := cpu.InstructionMap["X0Z1Q0"]
        return &inst
      default:
        fmt.Println("not implemented")
        return &Instruction{}
      }
}

func (cpu *Cpu) AddOpsToQueue(inst *Instruction) {
  for _, op := range inst.operations {
    cpu.ExecutionQueue.Push(&op)
  }
}

func (cpu *Cpu) FetchAndDecode() {
    // cold start: if CurrentOp doesn't exist,
    // don't inc PC. otherwise inc over the length of the opcode
    if (cpu.CurrentOpcode != Opcode{}) {
      // lookup nBytes
      nBytes := cpu.OpcodeToInstruction(cpu.CurrentOpcode).nBytes
      for i := uint8(0); i < nBytes; i++ {
        cpu.PC.inc()
      }
    }
    oc := ByteToOpcode(cpu.Bus.memory[cpu.PC.read()])
    inst := cpu.OpcodeToInstruction(oc)
    cpu.AddOpsToQueue(inst)
    cpu.CurrentOpcode = oc
}

func (gb *Cpu) Execute() {
  // TODO: timing
  for i := 0; i <= 10; i++ {
    // FetchAndDecode and AddOpsToQueue -> micro op1 -> micro op2 -> ... ->
    //   inc PC (depends on current Op) and FetchAndDecode and AddOpsToQueue
    // each pass of loop takes one cycle
    // the main loop first checks if we have something in the Queue
    // if we do, then we execute one item from queue and return to top of loop
    // if we don't, then we inc PC, execute FetchAndDecode, AddOpsToQueue, and thats one cycle
    // FetchAndDecode sets cpu.CurrentOp, which is stable and used until the queue is empty
    // at which point pc is inc'd and cpu.CurrentOp changed
    // cold start the queue will be empty and so will currentOp
    // we will inc PC, FetchAndDecode, AddOpsToQueue but we dont want to inc PC and can't
    // because we don't have a currentOp

    fmt.Printf("%d %x %d\n", i, gb.PC.read(), gb.ExecutionQueue.Length())
    if gb.ExecutionQueue.Length() < 1 {
        gb.FetchAndDecode()
    } else {
      microop := gb.ExecutionQueue.Pop()
      fmt.Printf("Executing %x\n", gb.CurrentOpcode.Full)
      (*microop)(gb)
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

  CurrentOpcode Opcode

  rpTable []Register16

  InstructionMap map[string]Instruction

  ExecutionQueue Fifo[*func(*Cpu)]

  Bus Bus
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

func NewGameBoy(romFilePath *string) *Cpu {
  gb := Cpu{}
 
  gb.clockSpeed = CLOCK_SPEED
  gb.rpTable = []Register16{gb.BC, gb.DE, gb.HL, gb.SP}

  gb.InstructionMap = MakeInstructionMap()

  ppu := Ppu{[160*144]uint8{}}
  bus := Bus{[64*1024]byte{}, [8192]byte{}, ppu}
  gb.Bus = bus
  gb.Bus.LoadROM(romFilePath)
  gb.Bus.LoadBootROM()
  return &gb
}
