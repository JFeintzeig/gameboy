package cpu

import (
  "fmt"
  "io/ioutil"
  "log"
  "runtime"
)

const CLOCK_SPEED uint64 = 4.19e6

type MemoryMapper struct {
  // TODO: memory mapping
  // is it bad to have slice of structs vs pointers to structs?
  // https://stackoverflow.com/questions/27622083/slices-of-structs-vs-slices-of-pointers-to-structs
  // suggests we're in a regime where it doesn't matter? let's see...
  // pointers is doable but makes code messier and harder to track
  // would also need to initialize
  memory [64*1024]Register8
}

func (mm *MemoryMapper) read(address uint16) uint8 {
  return mm.memory[address].read()
}

func (mm *MemoryMapper) write(address uint16, value uint8) {
  mm.memory[address].write(value)
}

type Bus struct {
  memory MemoryMapper
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
  for address, element := range data {
    // starts at 0x0, first 256 bytes
    // will be overwitten by boot rom
    gb.memory.write(uint16(address), element)
  }
}

func (gb *Bus) LoadBootROM() {
  data, err := ioutil.ReadFile("data/bootrom_dmg.gb")
  if err != nil {
    log.Fatal("can't find boot rom")
  }
  for address, element := range data {
    gb.memory.write(uint16(address), element)
  }
}

func (cpu *Cpu) OpcodeToInstruction(op Opcode) *Instruction {
      // TODO: improve? i can go from op -> stringcode if all
      // params are specified. if not, maybe i can fall back to
      // fewer and fewer params to catch them all?
      var inst Instruction
      if op.Prefixed {
        switch {
          case op.X == 0:
            inst = cpu.InstructionMap["CBX0"]
          case op.X == 1:
            inst = cpu.InstructionMap["CBX1"]
          default:
            err := fmt.Sprintf("CB-prefixed not implemented, instr: %v",op)
            panic(err)
            inst = Instruction{}
        }
        return &inst
      } else {
        switch {
        case (op.X == 0) && (op.Z == 0) && (op.Y == 0):
          inst = cpu.InstructionMap["X0Z0Y0"]
        case (op.X == 0) && (op.Z == 0) && (op.Y >= 4):
          inst = cpu.InstructionMap["X0Z0Ygte4"]
        case (op.X == 0) && (op.Z == 1) && (op.Q == 0):
          inst = cpu.InstructionMap["X0Z1Q0"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 1) && (op.Q == 1):
          inst = cpu.InstructionMap["X0Z2P1Q1"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 2) && (op.Q == 0):
          inst = cpu.InstructionMap["X0Z2P2Q0"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 3) && (op.Q == 0):
          inst = cpu.InstructionMap["X0Z2P3Q0"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 2) && (op.Q == 1):
          inst = cpu.InstructionMap["X0Z2P2Q1"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 3) && (op.Q == 1):
          inst = cpu.InstructionMap["X0Z2P3Q1"]
        case (op.X == 0) && (op.Z == 3):
          inst = cpu.InstructionMap["X0Z3"]
        case (op.X == 0) && (op.Z == 4):
          inst = cpu.InstructionMap["X0Z4"]
        case (op.X == 0) && (op.Z == 5):
          inst = cpu.InstructionMap["X0Z5"]
        case (op.X == 0) && (op.Z == 6):
          inst = cpu.InstructionMap["X0Z6"]
        case (op.X == 0) && (op.Z == 7) && (op.Y == 2):
          inst = cpu.InstructionMap["X0Z7Y2"]
        case (op.X == 1) && !(op.Y == 6 && op.Z == 6):
          inst = cpu.InstructionMap["X1"]
        case op.X == 2:
          inst = cpu.InstructionMap["X2"]
          return &inst
        case (op.X == 3) && (op.Y == 4) && (op.Z == 0):
          inst = cpu.InstructionMap["X3Y4Z0"]
        case (op.X == 3) && (op.Y == 6) && (op.Z == 0):
          inst = cpu.InstructionMap["X3Z0Y6"]
        case (op.X == 3) && (op.Z == 1) && (op.P == 0) && (op.Q == 1):
          inst = cpu.InstructionMap["X3Z1P0Q1"]
        case (op.X == 3) && (op.Z == 1) && (op.Q == 0):
          inst = cpu.InstructionMap["X3Z1Q0"]
        case (op.X == 3) && (op.Y == 4) && (op.Z == 2):
          inst = cpu.InstructionMap["X3Y4Z2"]
        case (op.X == 3) && (op.Z == 3) && (op.Y == 0):
          inst = cpu.InstructionMap["X3Z3Y0"]
        case (op.X == 3) && (op.Y == 6) && (op.Z == 3):
          inst = cpu.InstructionMap["X3Y6Z3"]
        case (op.X == 3) && (op.Y == 7) && (op.Z == 3):
          inst = cpu.InstructionMap["X3Y7Z3"]
        case (op.X == 3) && (op.Z == 5) && (op.Q == 0):
          inst = cpu.InstructionMap["X3Z5Q0"]
        case (op.X == 3) && (op.Z == 5) && (op.P == 0) && (op.Q == 1):
          inst = cpu.InstructionMap["X3Z5P0Q1"]
        case (op.X == 3) && (op.Z == 6):
          inst = cpu.InstructionMap["X3Z6"]
        default:
          err := fmt.Sprintf("not implemented, instr: %xv", op)
          panic(err)
          inst = Instruction{}
        }
        return &inst
      }
}

func (cpu *Cpu) AddOpsToQueue(inst *Instruction) {
  for _, op := range inst.operations {
    cpu.ExecutionQueue.Push(op)
  }
}

func (cpu *Cpu) FetchAndDecode() {
    // IncrementPC is initialized as `false` (b/c we want to start at 0x0)
    // and is also set to `false` by some instructions which set the PC
    // internally, e.g. `call`
    if (cpu.IncrementPC) {
      // lookup nBytes
      nBytes := cpu.OpcodeToInstruction(cpu.CurrentOpcode).nBytes
      for i := uint8(0); i < nBytes; i++ {
        cpu.PC.inc()
      }
    }
    // next time we want to increment it, unless told otherwise
    cpu.IncrementPC = true

    oc := ByteToOpcode(cpu.Bus.memory.read(cpu.PC.read()), false)
    // hmmm...if opcode isn't implemented, this sorta breaks because
    // the queue stays empty, FetchAndDecode() is called again
    // for the same PC, which reads the opcode but doesn't realize
    // its prefixed...will this ever be a problem when all opcodes
    // are implemented?
    if oc.Full == 0xCB {
      cpu.PC.inc()
      oc = ByteToOpcode(cpu.Bus.memory.read(cpu.PC.read()), true)
    }

    inst := cpu.OpcodeToInstruction(oc)
    cpu.AddOpsToQueue(inst)
    cpu.CurrentOpcode = oc
}

func (gb *Cpu) Execute() {
  // TODO: timing
  counter := 0
  for {
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

    fmt.Printf("%d %x %d %x %x %x\n", counter, gb.PC.read(), gb.ExecutionQueue.Length(), gb.SP.read(), gb.getFlagZ(), gb.F.read())
    fmt.Printf("%x %d %d %d %d %d\n", gb.CurrentOpcode.Full, gb.CurrentOpcode.X, gb.CurrentOpcode.Y, gb.CurrentOpcode.Z, gb.CurrentOpcode.P, gb.CurrentOpcode.Q)
    // potential alternative:
    // - FetchAndDecode doesn't increment PC
    // - after if statement, always pop+execute one f'n from the queue
    // - then if queue is empty at end, increment PC before next iteration through loop
    // * cold start will work: length is 0 and current opcode is blank, fetch+decode at
    //   current PC, then increment PC based on nBytes of opcode
    // * opcode that only takes 1 cycle will work fine: fetch+single execution stage
    //   both occur in same iteration of loop
    // * opcode that takes multiple cycles won't work fine: most have the fetch stage
    //   taking 1 cycle, and i assumed that would exist outside ExecutionQueue.
    // * opcode that jumps won't work fine b/c PC will still be incremented externally
    // * what happens to SetIME in this scheme? b/c first if statement only happens at
    //   cold start...
    if gb.ExecutionQueue.Length() < 1 {
        // probably put this into an interrupt handler eventually
        gb.SetIME()
        gb.FetchAndDecode()
        if gb.CurrentOpcode.Full == 0x40 {
          runtime.Breakpoint()
        }
    } else {
      microop := gb.ExecutionQueue.Pop()
      fmt.Printf("Executing %x, prefixed:%t\n", gb.CurrentOpcode.Full, gb.CurrentOpcode.Prefixed)
      microop(gb)
    }
    counter++
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

func (reg *Register16) readHi() uint8 {
  return reg.hi.value
}

func (reg *Register16) readLo() uint8 {
  return reg.lo.value
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

  rpTable []*Register16
  rp2Table []*Register16

  InstructionMap map[string]Instruction

  ExecutionQueue Fifo[func(*Cpu)]

  Bus Bus

  IncrementPC bool

  // Interrupts, maybe encapsulate this in a handler
  pcToSetIMEAfter uint16
  IME uint8
}

// TODO: validated Z, need to validate others
func (cpu *Cpu) getFlagZ() uint8 {
  return (cpu.F.read() & 0b10000000) >> 7
}

func (cpu *Cpu) setFlagZ() {
  cpu.F.write(cpu.F.read()|0b10000000)
}

func (cpu *Cpu) clearFlagZ() {
  cpu.F.write(cpu.F.read() & 0b01111111)
}

func (cpu *Cpu) getFlagN() uint8 {
  return (cpu.F.read() & 0b01000000) >> 6
}

func (cpu *Cpu) setFlagN() {
  cpu.F.write(cpu.F.read()|0b01000000)
}

func (cpu *Cpu) clearFlagN() {
  cpu.F.write(cpu.F.read() & 0b10111111)
}

func (cpu *Cpu) getFlagH() uint8 {
  return (cpu.F.read() & 0b00100000) >> 5
}

func (cpu *Cpu) setFlagH() {
  cpu.F.write(cpu.F.read()|0b00100000)
}

func (cpu *Cpu) clearFlagH() {
  cpu.F.write(cpu.F.read() & 0b11011111)
}


func (cpu *Cpu) getFlagC() uint8 {
  return (cpu.F.read() & 0b00010000) >> 4
}

func (cpu *Cpu) setFlagC() {
  cpu.F.write(cpu.F.read()|0b00010000)
}

func (cpu *Cpu) clearFlagC() {
  cpu.F.write(cpu.F.read() & 0b11101111)
}

func (cpu *Cpu) ReadNN() uint16 {
  // pc is location of current opcode
  // we want to pull 2 bytes _following_ pc
  // without changing PC
  // GB memory is little endian! so we switch the order here
  return (uint16(cpu.Bus.memory.read(cpu.PC.read()+2)) << 8) | uint16(cpu.Bus.memory.read(cpu.PC.read()+1))
}

func (cpu *Cpu) ReadN() uint8 {
  // pc is location of current opcode
  // we want to pull byte _following_ pc
  // without changing PC
  return cpu.Bus.memory.read(cpu.PC.read()+1)
}

func (cpu *Cpu) ReadD() int8 {
  // TODO: will this work??
  return int8(cpu.ReadN())
}

func (cpu *Cpu) SetIME() {
  if cpu.PC.read() > cpu.pcToSetIMEAfter {
    cpu.IME = 0x01
    cpu.pcToSetIMEAfter = 0xFFFF
  }
}

type Ppu struct {
  screen [160*144]uint8
}

// return pointer b/c some users write()/inc()/dec() the register
func (cpu *Cpu) GetRTableRegister(index uint8) *Register8 {
  if(index > 7) {
    panic("no register with index > 7")
  }
  switch index {
  case 0:
    return &(cpu.B)
  case 1:
    return &(cpu.C)
  case 2:
    return &(cpu.D)
  case 3:
    return &(cpu.E)
  case 4:
    return &(cpu.H)
  case 5:
    return &(cpu.L)
  case 6:
    return &(cpu.Bus.memory.memory[cpu.HL.read()])
  case 7:
    return &(cpu.A)
  }
  // should never happen
  return new(Register8)
}

func (cpu *Cpu) GetCCTableBool(index uint8) bool {
  if index > 3 {
    panic("cc table has no item with index > 3")
  }
  switch index {
  case 0:
    return cpu.getFlagZ() != 0x01
  case 1:
    return cpu.getFlagZ() == 0x01
  case 2:
    return cpu.getFlagC() != 0x01
  case 3:
    return cpu.getFlagC() == 0x01
  }
  return false // should never happen
}

func NewGameBoy(romFilePath *string, useBootRom bool) *Cpu {
  gb := Cpu{}
 
  gb.clockSpeed = CLOCK_SPEED
  // TODO: more verbose, but could change this to f'n with case switch
  gb.rpTable = []*Register16{&gb.BC, &gb.DE, &gb.HL, &gb.SP}
  gb.rp2Table = []*Register16{&gb.BC, &gb.DE, &gb.HL, &gb.AF}

  gb.InstructionMap = MakeInstructionMap()

  ppu := Ppu{[160*144]uint8{}}
  bus := Bus{MemoryMapper{}, [8192]byte{}, ppu}
  gb.Bus = bus
  gb.Bus.LoadROM(romFilePath)

  gb.IncrementPC = false

  // until video is implemented :(
  gb.Bus.memory.write(0xFF44, 0x90)
  // is this needed?
  // https://github.com/Gekkio/mooneye-test-suite#passfail-reporting
  gb.Bus.memory.write(0xFF02, 0xFF)

  if useBootRom {
    gb.Bus.LoadBootROM()
  } else {
    gb.A.write(0x01)
    gb.F.write(0xB0)
    gb.B.write(0x00)
    gb.C.write(0x13)
    gb.D.write(0x00)
    gb.E.write(0xD8)
    gb.H.write(0x01)
    gb.L.write(0x4D)
    gb.SP.write(0xFFFE)
    gb.PC.write(0x0100)
  }
  return &gb
}
