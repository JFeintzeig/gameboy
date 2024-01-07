package cpu

import (
  "encoding/hex"
  "fmt"
  "time"
)

const ClockSpeed uint64 = 1048576 // M-cycle

type Timers struct {
  bus Mediator

  tima Register8
  tma Register8

  divCounter uint16
  timaMask uint16
  timaEnabled bool
  justOverflowed bool
  afterJustOverflowed bool
}

func (t *Timers) read(address uint16) uint8 {
  if address == 0xFF04 {
    return t.readDiv()
  } else if address == 0xFF05 {
    return t.tima.read()
  } else if address == 0xFF06 {
    return t.tma.read()
  } else if address == 0xFF07 {
    return t.readTAC()
  } else {
    panic("address not in timers")
  }
}

func (t *Timers) write(address uint16, value uint8) {
  if address == 0xFF04 {
    t.writeDiv(value)
  } else if address == 0xFF05 {
    t.writeTima(value)
  } else if address == 0xFF06 {
    t.writeTma(value)
  } else if address == 0xFF07 {
    t.writeTAC(value)
  } else {
    panic("address not in timers")
  }
}

func (t *Timers) writeTma(value uint8) {
  t.tma.write(value)

  // if TMA is updated in same cycle as it's
  // written to TIMA, grab the new value
  if t.afterJustOverflowed {
    t.tima.write(t.tma.read())
  }
}

func (t *Timers) readTAC() uint8 {
  var cs uint8
  switch t.timaMask {
    case 1023: cs = 0x00
    case 15: cs = 0x01
    case 63: cs = 0x02
    case 255: cs = 0x03
  }

  return SetBitBool(cs, 2, t.timaEnabled)
}

func (t *Timers) writeTAC(value uint8) {
  t.timaEnabled = GetBitBool(value, 2)
  switch (value & 0x03) {
    case 0x00:
      t.timaMask = 1023
    case 0x01:
      t.timaMask = 15
    case 0x02:
      t.timaMask = 63
    case 0x03:
      t.timaMask = 255
  }
}

func (t *Timers) readDiv() uint8 {
 return uint8(t.divCounter >> 8)
}

func (t *Timers) writeDiv(value uint8) {
  // TODO: deal with edge cases
  // TODO: implement STOP instruction
  // if MSB of timer changes from 1 to 0, tima increases
  msb := t.divCounter & t.timaMask
  switch t.timaMask {
    case 15: msb = msb >> 3
    case 63: msb = msb >> 5
    case 255: msb = msb >> 7
    case 1023: msb = msb >> 9
  }

  if t.timaEnabled && msb == 1 {
    t.tima.inc()
  }
  t.divCounter = 0
}

func (t *Timers) writeTima(value uint8) {
  // prevent write during cycle [B]
  if !t.afterJustOverflowed {
    t.tima.write(value)
  }
  if t.justOverflowed {
    // write during cycle [A] prevents interrupt flag and LD TIMA, TMA
    t.justOverflowed = false
  }
}

// TODO: other weird edge cases w/writing to DIV or TAC
func (t *Timers) doCycle() {
  //fmt.Printf("counter: %d, div: %X, tima: %X, mask: %X, enabled: %t, tma: %X, tac: %X, int: %X\n",t.divCounter, t.readDiv(), t.tima.read(), t.timaMask, t.timaEnabled, t.tma.read(), t.readTAC(), t.bus.ReadFromBus(0xFF0F))
  t.divCounter += 4

  // order of reseting vs. setting this matter, needs to be
  // true for 1 cycle
  if t.afterJustOverflowed {
    t.afterJustOverflowed = false
  }

  // fire interrupt + reset TIMA the cycle after it overflowed
  if t.justOverflowed {
    //fmt.Printf("timer int tima:%02X timaEnabled:%t timaMask: %16b tma:%02X div:%04X\n", t.tima.read(), t.timaEnabled, t.timaMask, t.tma.read(), t.divCounter)
    interruptFlags := t.bus.ReadFromBus(0xFF0F)
    interruptFlags |= 0x4
    t.bus.WriteToBus(0xFF0F, interruptFlags)
    t.tima.write(t.tma.read())

    t.justOverflowed = false
    t.afterJustOverflowed = true
  }

  if (t.divCounter & t.timaMask) == 0  && t.timaEnabled {
    t.tima.inc()
    if t.tima.read() == 0x0 {
      t.justOverflowed = true
    }
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
          case op.X == 2:
            inst = cpu.InstructionMap["CBX2"]
          case op.X == 3:
            inst = cpu.InstructionMap["CBX3"]
          default:
            err := fmt.Sprintf("CB-prefixed not implemented, instr: %v",op)
            panic(err)
        }
        return &inst
      } else {

        switch {
        case (op.X == 0) && (op.Z == 0) && (op.Y == 0):
          inst = cpu.InstructionMap["X0Z0Y0"]
        case (op.X == 0) && (op.Z == 0) && (op.Y == 1):
          inst = cpu.InstructionMap["X0Z0Y1"]
        case (op.X == 0) && (op.Z == 0) && (op.Y == 2):
          inst = cpu.InstructionMap["X0Z0Y2"]
        case (op.X == 0) && (op.Z == 0) && (op.Y == 3):
          inst = cpu.InstructionMap["X0Z0Y3"]
        case (op.X == 0) && (op.Z == 0) && (op.Y >= 4):
          inst = cpu.InstructionMap["X0Z0Ygte4"]
        case (op.X == 0) && (op.Z == 1) && (op.Q == 0):
          inst = cpu.InstructionMap["X0Z1Q0"]
        case (op.X == 0) && (op.Z == 1) && (op.Q == 1):
          inst = cpu.InstructionMap["X0Z1Q1"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 0) && (op.Q == 0):
          inst = cpu.InstructionMap["X0Z2P0Q0"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 1) && (op.Q == 0):
          inst = cpu.InstructionMap["X0Z2P1Q0"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 2) && (op.Q == 0):
          inst = cpu.InstructionMap["X0Z2P2Q0"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 3) && (op.Q == 0):
          inst = cpu.InstructionMap["X0Z2P3Q0"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 0) && (op.Q == 1):
          inst = cpu.InstructionMap["X0Z2P0Q1"]
        case (op.X == 0) && (op.Z == 2) && (op.P == 1) && (op.Q == 1):
          inst = cpu.InstructionMap["X0Z2P1Q1"]
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
        case (op.X == 0) && (op.Z == 7) && (op.Y <= 3):
          inst = cpu.InstructionMap["X0Z7Ylte3"]
        case (op.X == 0) && (op.Z == 7) && (op.Y == 4):
          inst = cpu.InstructionMap["X0Z7Y4"]
        case (op.X == 0) && (op.Z == 7) && (op.Y == 5):
          inst = cpu.InstructionMap["X0Z7Y5"]
        case (op.X == 0) && (op.Z == 7) && (op.Y == 6):
          inst = cpu.InstructionMap["X0Z7Y6"]
        case (op.X == 0) && (op.Z == 7) && (op.Y == 7):
          inst = cpu.InstructionMap["X0Z7Y7"]
        case (op.X == 1) && !(op.Z == 6 && op.Y == 6):
          inst = cpu.InstructionMap["X1"]
        case (op.X == 1) && (op.Z == 6) && (op.Y == 6):
          inst = cpu.InstructionMap["X1Z6Y6"]
        case op.X == 2:
          inst = cpu.InstructionMap["X2"]
          return &inst
        case (op.X == 3) && (op.Z == 0) && (op.Y <= 3):
          inst = cpu.InstructionMap["X3Z0Ylte3"]
        case (op.X == 3) && (op.Z == 0) && (op.Y == 4):
          inst = cpu.InstructionMap["X3Z0Y4"]
        case (op.X == 3) && (op.Z == 0) && (op.Y == 5):
          inst = cpu.InstructionMap["X3Z0Y5"]
        case (op.X == 3) && (op.Z == 0) && (op.Y == 6):
          inst = cpu.InstructionMap["X3Z0Y6"]
        case (op.X == 3) && (op.Z == 0) && (op.Y == 7):
          inst = cpu.InstructionMap["X3Z0Y7"]
        case (op.X == 3) && (op.Z == 1) && (op.Q == 0):
          inst = cpu.InstructionMap["X3Z1Q0"]
        case (op.X == 3) && (op.Z == 1) && (op.Q == 1) && (op.P == 0):
          inst = cpu.InstructionMap["X3Z1Q1P0"]
        case (op.X == 3) && (op.Z == 1) && (op.Q == 1) && (op.P == 1):
          inst = cpu.InstructionMap["X3Z1Q1P1"]
        case (op.X == 3) && (op.Z == 1) && (op.Q == 1) && (op.P == 2):
          inst = cpu.InstructionMap["X3Z1Q1P2"]
        case (op.X == 3) && (op.Z == 1) && (op.Q == 1) && (op.P == 3):
          inst = cpu.InstructionMap["X3Z1Q1P3"]
        case (op.X == 3) && (op.Z == 2) && (op.Y <= 3):
          inst = cpu.InstructionMap["X3Z2Ylte3"]
        case (op.X == 3) && (op.Z == 2) && (op.Y == 4):
          inst = cpu.InstructionMap["X3Z2Y4"]
        case (op.X == 3) && (op.Z == 2) && (op.Y == 5):
          inst = cpu.InstructionMap["X3Z2Y5"]
        case (op.X == 3) && (op.Z == 2) && (op.Y == 6):
          inst = cpu.InstructionMap["X3Z2Y6"]
        case (op.X == 3) && (op.Z == 2) && (op.Y == 7):
          inst = cpu.InstructionMap["X3Z2Y7"]
        case (op.X == 3) && (op.Z == 3) && (op.Y == 0):
          inst = cpu.InstructionMap["X3Z3Y0"]
        case (op.X == 3) && (op.Y == 6) && (op.Z == 3):
          inst = cpu.InstructionMap["X3Y6Z3"]
        case (op.X == 3) && (op.Y == 7) && (op.Z == 3):
          inst = cpu.InstructionMap["X3Y7Z3"]
        case (op.X == 3) && (op.Z == 4) && (op.Y <= 3):
          inst = cpu.InstructionMap["X3Z4Ylte3"]
        case (op.X == 3) && (op.Z == 5) && (op.Q == 0):
          inst = cpu.InstructionMap["X3Z5Q0"]
        case (op.X == 3) && (op.Z == 5) && (op.P == 0) && (op.Q == 1):
          inst = cpu.InstructionMap["X3Z5P0Q1"]
        case (op.X == 3) && (op.Z == 6):
          inst = cpu.InstructionMap["X3Z6"]
        case (op.X == 3) && (op.Z == 7):
          inst = cpu.InstructionMap["X3Z7"]
        default:
          fmt.Printf("unimplemented opcode: PC:%04X SP:%04X OC:%v\n", cpu.PC.read(), cpu.SP.read(), cpu.CurrentOpcode)
          err := fmt.Sprintf("not implemented, instr: %v", op)
          panic(err)
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
    // if isHalted, we don't increment PC, so we keep executing the HALT instr
    if cpu.IncrementPC && !cpu.isHalted {
      // lookup nBytes
      nBytes := cpu.OpcodeToInstruction(cpu.CurrentOpcode).nBytes
      for i := uint8(0); i < nBytes; i++ {
        cpu.PC.inc()
      }
    }
    cpu.IncrementPC = true

    oc := ByteToOpcode(cpu.Bus.ReadFromBus(cpu.PC.read()), false)

    if oc.Full == 0xCB {
      cpu.PC.inc()
      oc = ByteToOpcode(cpu.Bus.ReadFromBus(cpu.PC.read()), true)
    }

    inst := cpu.OpcodeToInstruction(oc)
    cpu.AddOpsToQueue(inst)
    cpu.CurrentOpcode = oc
}

func (cpu *Cpu) LogSerial() {
  if cpu.Bus.ReadFromBus(0xFF02) != 0x0 {
    serial := cpu.Bus.ReadFromBus(0xFF01)
    hexString := fmt.Sprintf("%X",serial)
    ascii, err := hex.DecodeString(hexString)
    if err != nil {
      fmt.Printf("\nErr: %x\n",serial)
    } else {
      fmt.Printf("%s",ascii)
    }
    cpu.Bus.WriteToBus(0xFF02, 0x0)
  }
}

// https://gbdev.io/pandocs/Interrupts.html#interrupts
func (cpu *Cpu) DoInterrupts() {
  cpu.justDidInterrupt = false
  if !cpu.IME {
    return
  }

  // hardcoded addresses of interrupt service routines
  jumpFunctions := []func(*Cpu){
    func (cpu *Cpu) {cpu.PC.write(0x40)},
    func (cpu *Cpu) {cpu.PC.write(0x48)},
    func (cpu *Cpu) {cpu.PC.write(0x50)},
    func (cpu *Cpu) {cpu.PC.write(0x58)},
    func (cpu *Cpu) {cpu.PC.write(0x60)},
  }

  interruptEnable := cpu.Bus.ReadFromBus(0xFFFF)
  interruptFlags := cpu.Bus.ReadFromBus(0xFF0F)

  interruptsToService := interruptFlags & interruptEnable

  // priority from bit 0 -> 3
  for _, index := range []uint8{0,1,2,3} {
    isRequested := (interruptsToService >> index) & 0x01
    if isRequested == 0x01 {
      //fmt.Printf("serving interrupt IME:%t IE:%08b IF:%08b PC:%04X GC:%d\n", cpu.IME, cpu.Bus.ReadFromBus(IE), cpu.Bus.ReadFromBus(IF), cpu.PC.read(), cpu.globalCounter)
      cpu.justDidInterrupt = true
      // reset flag bit
      mask := uint8(1 << index)
      mask = ^mask
      cpu.Bus.WriteToBus(IF, interruptFlags & mask)
      // reset IME
      cpu.IME = false
      // push handling routine to queue
      // 5 cycles: 2 no_op, push PC to stack, set PC to hardcoded address
      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(call_push_hi)
      cpu.ExecutionQueue.Push(call_push_lo)
      cpu.ExecutionQueue.Push(jumpFunctions[index])
      cpu.IncrementPC = false
      return
    }
  }
}

func (cpu *Cpu) Execute(forever bool, nCyles uint64) {
  var counter uint64 = 0
  var loopsPerFrame uint64 = cpu.ClockSpeed / 60
  timePerFrame := time.Duration(16.74 * 1e6)
  start := time.Now()

  for {
    cpu.DoInterrupts()
    cpu.LogSerial()
    // TODO: refactor all this into Bus.doCycle()
    cpu.Bus.timers.doCycle()
    cpu.Bus.joypad.doCycle()
    cpu.Bus.doCycle()
    // TODO: need to figure out _when_ to do interrupts!!!
    // Timers -> Int -> PPU -> CPU: acid2 has no background, appears to hit LC_08 but not LC_10
    //    why? looks like i never get LYC == LY int again, even though PPU appears to fire it by
    //    setting IF. confirm in PPU logging that IE and IF look good at LYC=16. weird thing
    //    is that the LYC == LY @ 08 interrupt appears to work as desired. so what's wrong?
    //    ohh maybe RETI not setting IME correctly?
    //    OK SetIME is broken: i assume PC will be incremented but RETI goes somewhere completely different,
    //    which actually just runs HALT in a loop until an interrupt is fired+handled, but interrupt will never
    //    be handled because IME is still false because PC was never incremented. need to refactor SetIME and EI
    //    and RETI so it waits one instruction, not a specific PC
    //    
    // Timers -> PPU -> Int -> CPU: acid2 stuck in HALT after jumping to LC_08
    cpu.Bus.ppu.doCycle()

    if cpu.ExecutionQueue.Length() < 1 {
        cpu.SetIME()
        cpu.FetchAndDecode()
    }

    if !forever && cpu.globalCounter == nCyles {
      break
    }

    microop := cpu.ExecutionQueue.Pop()
    microop(cpu)
    counter++

    // time true-up once per frame
    if !cpu.fast && counter > loopsPerFrame {
      delta := time.Now().Sub(start)
      if delta < timePerFrame {
        time.Sleep(timePerFrame - delta) // remaining time
      }
      counter = 0
      start = time.Now()
    }

    cpu.globalCounter += 1
  }
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
  ClockSpeed uint64

  CurrentOpcode Opcode

  rpTable []*Register16
  rp2Table []*Register16

  InstructionMap map[string]Instruction

  ExecutionQueue Fifo[func(*Cpu)]

  Bus *Bus

  IncrementPC bool

  // Interrupts, maybe encapsulate this in a handler
  IMECountdown int8
  IME bool
  isHalted bool
  justDidInterrupt bool

  fast bool

  globalCounter uint64
}

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
  return (uint16(cpu.Bus.ReadFromBus(cpu.PC.read()+2)) << 8) | uint16(cpu.Bus.ReadFromBus(cpu.PC.read()+1))
}

func (cpu *Cpu) ReadN() uint8 {
  // pc is location of current opcode
  // we want to pull byte _following_ pc
  // without changing PC
  return cpu.Bus.ReadFromBus(cpu.PC.read()+1)
}

func (cpu *Cpu) ReadD() int8 {
  return int8(cpu.ReadN())
}

func (cpu *Cpu) SetIME() {
  if cpu.IMECountdown == 0 {
    cpu.IME = true
  }
  // gte so it goes to -1 and then stays there until reset
  if cpu.IMECountdown >= 0 {
    cpu.IMECountdown -= 1
  }
}

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
    panic("can't get (HL) using GetRTableRegister")
  case 7:
    return &(cpu.A)
  }
  return new(Register8) // should never happen
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

func NewRegister16(hi *Register8, lo *Register8) Register16 {
  reg16 := Register16{}
  reg16.hi = hi
  reg16.lo = lo
  return reg16
}

func NewGameBoy(romFilePath *string, useBootRom bool, fast bool) *Cpu {
  gb := Cpu{}

  gb.AF = NewRegister16(&gb.A, &gb.F)
  gb.BC = NewRegister16(&gb.B, &gb.C)
  gb.DE = NewRegister16(&gb.D, &gb.E)
  gb.HL = NewRegister16(&gb.H, &gb.L)
  dummySP_S := Register8{}
  dummySP_P := Register8{}
  dummyPC_P := Register8{}
  dummyPC_C := Register8{}
  gb.SP = NewRegister16(&dummySP_S, &dummySP_P)
  gb.PC = NewRegister16(&dummyPC_P, &dummyPC_C)

  gb.ClockSpeed = ClockSpeed
  gb.rpTable = []*Register16{&gb.BC, &gb.DE, &gb.HL, &gb.SP}
  gb.rp2Table = []*Register16{&gb.BC, &gb.DE, &gb.HL, &gb.AF}
  gb.InstructionMap = MakeInstructionMap()

  // returns a *Bus
  bus := NewBus(*romFilePath, useBootRom)
  gb.Bus = bus

  gb.IncrementPC = false
  gb.fast = fast

  if !useBootRom {
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
