package cpu

//import (
//  "fmt"
//)

// Opcode is the parsed octal representation of a byte
// https://gb-archive.github.io/salvage/decoding_gbz80_opcodes/Decoding%20Gamboy%20Z80%20Opcodes.html
type Opcode struct {
  Full uint8
  X uint8
  Y uint8
  Z uint8
  P uint8
  Q uint8
  Prefixed bool
}

// TODO: document this logic and these magic numbers
func ByteToOpcode(oneByte uint8, prefixed bool) Opcode {
  op := Opcode{
    Full: oneByte,
    X: uint8(0b11000000 & oneByte) >> 6,
    Y: uint8(0b111000 & oneByte) >> 3,
    Z: uint8(0b111 & oneByte),
    P: uint8(0b110000 & oneByte) >> 4,
    Q: (uint8(0b1000 & oneByte) >> 3) % 2,
    Prefixed: prefixed,
  }
  return op
}

// Instruction contains the actual execution logic for
// one or more instructions
type Instruction struct {
  name string
  nBytes uint8
  operations []func(*Cpu)
}

func MakeInstructionMap() map[string]Instruction {
  // the keys in this map are just my internal
  // names based on the X/Y/Z/P/Q's we need to
  // match on, since its a many -> one mapping
  // of opcodes to actual exection instructions.
  instructionMap := make(map[string]Instruction)

  // X=0, Z=1, Q=0
  x0z1q0_1 := func (cpu *Cpu) {
    nn := cpu.ReadNN()
    (*cpu.rpTable[cpu.CurrentOpcode.P]).writeLo(nn)
  }
  
  x0z1q0_2 := func (cpu *Cpu) {
    nn := cpu.ReadNN()
    (*cpu.rpTable[cpu.CurrentOpcode.P]).writeHi(nn)
  }

  instructionMap["X0Z1Q0"] = Instruction{
    "LD rp[p] nn",
    3,
    []func(*Cpu){x0z1q0_1, x0z1q0_2},
  }

  // X=0, Z=2, P=3, Q=0
  x0z2q0p3_1 := func (cpu *Cpu) {
    cpu.Bus.memory.write(cpu.HL.read(), cpu.A.read())
    cpu.HL.dec()
  }

  instructionMap["X0Z2P3Q0"] = Instruction {
   "LDD (HL) A",
   1,
   []func(*Cpu){x0z2q0p3_1},
  }

  // X=2
  x2_1 := func (cpu *Cpu) {
    register := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
    a := cpu.A.read()
    b := register.read()
    var result uint8
    y := cpu.CurrentOpcode.Y
    switch y {
    case 0: // ADD A,
      result = a + b
    case 1: // ADC A,
      result = a + b + cpu.getFlagC()
    case 2: // SUB
      result = a - b
    case 3: // SBC A,
      result = a - b - cpu.getFlagC()
    case 4: // AND
      result = a & b
    case 5: // XOR
      result = a ^ b
    case 6: // OR
      result = a | b
    case 7: // CP
      result = a - b
    }

    // set flags
    if result == 0 {
      cpu.setFlagZ()
    } else {
      cpu.clearFlagZ()
    }

    if y == 2 || y == 3 || y == 7 {
      cpu.setFlagN()
    } else {
      cpu.clearFlagN()
    }

    if (y == 0 || y == 1) && (result < a) {
      cpu.setFlagC()
    } else if (y == 2 || y == 3 || y == 7) && (result > a) {
      cpu.setFlagC()
    } else {
      cpu.clearFlagC()
    }

    if ( y == 0 || y == 1) && ((result & 0x0f) < (a & 0x0f)) {
      cpu.setFlagH()
    } else if (y == 2 || y == 3 || y == 7) && ((result & 0x0f) > (a & 0x0f)) {
      cpu.setFlagH()
    } else if y == 4 {
      // really?
      cpu.setFlagH()
    } else {
      cpu.clearFlagH()
    }

    // store result
    if y != 7 {
      cpu.A.write(result)
    }
    return
  }

 // TODO: if instructions in same group
 // have different nBytes, i have problem
 // TODO: if instructions should only last
 // 1 cycle, i have problem b/c i have separate
 // cycle for fetch and microops
  instructionMap["X2"] = Instruction{
    "alu[y] r[z]",
    1,
    []func(*Cpu){x2_1},
  }

  cbx1_1 := func (cpu *Cpu) {
    oc := cpu.CurrentOpcode
    register := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
    result := (register.read() >> oc.Y) & 0x01
    if result == 0 {
      cpu.setFlagZ()
    }
    cpu.clearFlagN()
    cpu.setFlagH()
  }

  instructionMap["CBX1"] = Instruction{
    "bit y, r[z]",
    2,
    []func(*Cpu){cbx1_1},
  }

  x3y7z3 := func (cpu *Cpu) {
    // EI: set IME flag _after_ the following instruction
    // we write current PC to this variable
    // when PC is greater than this variable, we will set
    // the flag and reset this value to 0xFFFF (the max possible PC)
    // execution logic:
    // - we set the pcToSet variable, but PC is not yet incremented
    // - the next Execute() loop first tests pcToSet, current PC is
    //   not greater, so it fetches+decodes the next instruction
    //   and increments PC
    // - next N loops through Execute() execute that instruction,
    //   based on functions in the queue
    // - when queue is empty, we first call SetIME(), which now
    //   sets the flag because PC has been incremented. we then
    //   further increment PC +decode + execute another opcode after.
    cpu.pcToSetIMEAfter = cpu.PC.read()
  }

  instructionMap["X3Y7Z3"] = Instruction{
    "ei",
    1,
    []func(*Cpu){x3y7z3},
  }

  x0z6_1 := func (cpu *Cpu) {
    cpu.GetRTableRegister(cpu.CurrentOpcode.Y).write(cpu.ReadN())
  }

  instructionMap["X0Z6"] = Instruction{
    "ld r[y], n",
    2,
    []func(*Cpu){x0z6_1},
  }

  x3y4z2_1 := func (cpu *Cpu) {
    cpu.Bus.memory.write(0xFF00 + uint16(cpu.C.read()), cpu.A.read())
  }

  instructionMap["X3Y4Z2"] = Instruction{
    "ld [0xFF00 + c], a",
    1,
    []func(*Cpu){x3y4z2_1},
  }

  x0z4_1 := func (cpu *Cpu) {
    reg := cpu.GetRTableRegister(cpu.CurrentOpcode.Y)
    reg.inc()
  }

  instructionMap["X0Z4"] = Instruction{
    "inc r[y]",
    1,
    []func(*Cpu){x0z4_1},
  }

  x3y6z3_1 := func (cpu *Cpu) {
    cpu.IME = 0x0
  }

  instructionMap["X3Y6Z3"] = Instruction{
    "di",
    1,
    []func(*Cpu){x3y6z3_1},
  }

  x1_1:= func (cpu *Cpu) {
    // oh no, if it gets [HL] that's an extra 1-2 cycles compared
    // to other values of Y / Z :(
    from := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
    to := cpu.GetRTableRegister(cpu.CurrentOpcode.Y)
    to.write(from.read())
  }

  instructionMap["X1"] = Instruction{
    "ld r[y] r[z]",
    1,
    []func(*Cpu){x1_1},
  }

  x3y4z0_1 := func (cpu *Cpu) {
    // this is not used, we just split
    // this up to satisfy timing
    _ = cpu.ReadN()
  }

  x3y4z0_2 := func (cpu *Cpu) {
    n := cpu.ReadN()
    cpu.Bus.memory.write(0xFF00 + uint16(n), cpu.A.read())
  }

  instructionMap["X3Y4Z0"] = Instruction{
    "ld [0xFF00+u8], A",
    2,
    []func(*Cpu){x3y4z0_1, x3y4z0_2},
  }

  x3z5q0_1 := func (cpu *Cpu) {
    // just taking up time
    return
  }

  x3z5q0_2 := func (cpu *Cpu) {
    cpu.SP.dec()
    cpu.Bus.memory.write(cpu.SP.read(), cpu.rp2Table[cpu.CurrentOpcode.P].readHi())
  }

  x3z5q0_3 := func (cpu *Cpu) {
    cpu.SP.dec()
    cpu.Bus.memory.write(cpu.SP.read(), cpu.rp2Table[cpu.CurrentOpcode.P].readLo())
  }

  instructionMap["X3Z5Q0"] = Instruction{
    "push rp2[p]",
    1,
    []func(*Cpu){x3z5q0_1,x3z5q0_2,x3z5q0_3},
  }

  return instructionMap
}
