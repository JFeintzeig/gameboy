package cpu

import (
//  "fmt"
//  "runtime"
)

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

func (cpu *Cpu) DoAluInstruction(a uint8, b uint8) {
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

  // X=0, Z=2, P=2, Q=0
  x0z2q0p2_1 := func (cpu *Cpu) {
    cpu.Bus.memory.write(cpu.HL.read(), cpu.A.read())
    cpu.HL.inc()
  }

  instructionMap["X0Z2P2Q0"] = Instruction {
   "LDI (HL) A",
   1,
   []func(*Cpu){x0z2q0p2_1},
  }

  // X=0, Z=2, P=3, Q=1
  x0z2q1p3_1 := func (cpu *Cpu) {
    cpu.A.write(cpu.Bus.memory.read(cpu.HL.read()))
    cpu.HL.dec()
  }

  instructionMap["X0Z2P3Q1"] = Instruction {
   "LDD A (HL)",
   1,
   []func(*Cpu){x0z2q1p3_1},
  }

  // X=0, Z=2, P=2, Q=1
  x0z2q1p2_1 := func (cpu *Cpu) {
    cpu.A.write(cpu.Bus.memory.read(cpu.HL.read()))
    cpu.HL.inc()
  }

  instructionMap["X0Z2P2Q1"] = Instruction {
   "LDI A (HL)",
   1,
   []func(*Cpu){x0z2q1p2_1},
  }

  // X=2
  x2_1 := func (cpu *Cpu) {
    register := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
    a := cpu.A.read()
    b := register.read()

    cpu.DoAluInstruction(a, b)
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
    "BIT y, r[z]",
    1,
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
    "EI",
    1,
    []func(*Cpu){x3y7z3},
  }

  x0z6_1 := func (cpu *Cpu) {
    cpu.GetRTableRegister(cpu.CurrentOpcode.Y).write(cpu.ReadN())
  }

  instructionMap["X0Z6"] = Instruction{
    "LD r[y], N",
    2,
    []func(*Cpu){x0z6_1},
  }

  x3y4z2_1 := func (cpu *Cpu) {
    cpu.Bus.memory.write(0xFF00 + uint16(cpu.C.read()), cpu.A.read())
  }

  instructionMap["X3Y4Z2"] = Instruction{
    "LD [0xFF00 + c], A",
    1,
    []func(*Cpu){x3y4z2_1},
  }

  x3y6z3_1 := func (cpu *Cpu) {
    cpu.IME = 0x0
  }

  instructionMap["X3Y6Z3"] = Instruction{
    "DI",
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
    "LD r[y] r[z]",
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
    "LD [0xFF00+u8], A",
    2,
    []func(*Cpu){x3y4z0_1, x3y4z0_2},
  }

  no_op := func (cpu *Cpu) {
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
    "PUSH rp2[p]",
    1,
    []func(*Cpu){no_op,x3z5q0_2,x3z5q0_3},
  }

  x3z2p1q1_1 := func (cpu *Cpu) {
    cpu.A.write(cpu.Bus.memory.read(cpu.DE.read()))
  }

  instructionMap["X0Z2P1Q1"] = Instruction{
    "LD A, [DE]",
    1,
    []func(*Cpu){x3z2p1q1_1},
  }

  // combining Q=0 and Q=1 into one function
  x0z3_1 := func (cpu *Cpu) {
    reg := cpu.rpTable[cpu.CurrentOpcode.P]
    if cpu.CurrentOpcode.Q == 0 {
      reg.inc()
    } else if cpu.CurrentOpcode.Q == 1 {
      reg.dec()
    }
  }

  instructionMap["X0Z3"] = Instruction{
    "INC rp[p]",
    1,
    []func(*Cpu){x0z3_1},
  }

  x0z0ygte4_1 := func (cpu *Cpu) {
      // function to do the jump
      x0z0ygte4_2 := func (cpu *Cpu) {
        // TODO: does this work?? signed and unsigned
        // confusion
        newPC := cpu.PC.read() + uint16(cpu.ReadD())
        cpu.PC.write(newPC)
        // NB: the relative jump is relative to the
        // instruction _after_ this one. so we don't
        // set IncrementPC to false, so FetchAndDecode
        // will increment this by 2 before decoding
        // the next instruction. this is super convoluted
      }

    cond := cpu.GetCCTableBool(cpu.CurrentOpcode.Y-4)
    if (cond) {
      // will this break shit? def. feels like
      // crossing an encapsulation boundary at least
      cpu.ExecutionQueue.Push(x0z0ygte4_2)
    } else {
      return
    }
  }

  instructionMap["X0Z0Ygte4"] = Instruction{
    "JR cc[y-4], d",
    2,
    []func(*Cpu){x0z0ygte4_1},
  }

  x3z6_1 := func (cpu *Cpu) {
    a := cpu.A.read()
    b := cpu.ReadN()
    cpu.DoAluInstruction(a, b)
  }

  instructionMap["X3Z6"] = Instruction{
    "alu[y] n",
    2,
    []func(*Cpu){x3z6_1},
  }

  call_push_hi := func (cpu *Cpu) {
    cpu.SP.dec()
    cpu.Bus.memory.write(cpu.SP.read(), cpu.PC.readHi())
  }

  call_push_lo_and_jump := func (cpu *Cpu) {
    cpu.SP.dec()
    cpu.Bus.memory.write(cpu.SP.read(), cpu.PC.readLo())
    cpu.PC.write(cpu.ReadNN())
    cpu.IncrementPC = false
  }

  instructionMap["X3Z5P0Q1"] = Instruction{
    "CALL NN",
    3,
    []func(*Cpu){no_op, no_op, no_op, call_push_hi, call_push_lo_and_jump},
  }

  x3z0y6_1 := func(cpu *Cpu) {
    _ = cpu.ReadN()
  }

  x3z0y6_2 := func(cpu *Cpu) {
    cpu.A.write(cpu.Bus.memory.read(0xFF00 | uint16(cpu.ReadN())))
  }

  instructionMap["X3Z0Y6"] = Instruction{
    "LD A, [0xFF00+n]",
    2,
    []func(*Cpu){x3z0y6_1, x3z0y6_2},
  }

  x0z5_1 := func(cpu *Cpu) {
    //if (cpu.C.read() == 1 && cpu.E.read() == 1) {
    //  runtime.Breakpoint()
    //}
    reg := cpu.GetRTableRegister(cpu.CurrentOpcode.Y)
    reg.dec()
    val := reg.read()

    if val == 0 {
      cpu.setFlagZ()
    } else {
      cpu.clearFlagZ()
    }

    cpu.setFlagN()

    if ((val & 0x0f) > ((val+1) & 0x0f)) {
      cpu.setFlagH()
    } else {
      cpu.clearFlagH()
    }
  }

  instructionMap["X0Z5"] = Instruction{
    "DEC r[y]",
    1,
    []func(*Cpu){x0z5_1},
  }

  x0z4_1 := func(cpu *Cpu) {
    reg := cpu.GetRTableRegister(cpu.CurrentOpcode.Y)
    reg.inc()
    val := reg.read()

    if val == 0 {
      cpu.setFlagZ()
    } else {
      cpu.clearFlagZ()
    }

    cpu.clearFlagN()

    if ((val & 0x0f) < ((val-1) & 0x0f)) {
      cpu.setFlagH()
    } else {
      cpu.clearFlagH()
    }
  }

  instructionMap["X0Z4"] = Instruction{
    "INC r[y]",
    1,
    []func(*Cpu){x0z4_1},
  }

  x3z1p0q1_1 := func(cpu *Cpu) {
    lower := cpu.Bus.memory.read(cpu.SP.read())
    cpu.SP.inc()
    upper := cpu.Bus.memory.read(cpu.SP.read())

    cpu.PC.write(uint16(upper) << 8 | uint16(lower))
    // we want the CPU to operate starting from this
    // new PC, and not increment again in the Fetch loop
    cpu.IncrementPC = false
  }

  instructionMap["X3Z1P0Q1"] = Instruction{
    "RET",
    1,
    []func(*Cpu){no_op, no_op,x3z1p0q1_1},
  }

  instructionMap["X0Z0Y0"] = Instruction{
    "NOP",
    1,
    []func(*Cpu){no_op},
  }

  x3z3y0_1 := func(cpu *Cpu) {
    nn := cpu.ReadNN()
    cpu.PC.write(nn)
    cpu.IncrementPC = false
  }

  instructionMap["X3Z3Y0"] = Instruction{
    "JP NN",
    3,
    []func(*Cpu){no_op, no_op, x3z3y0_1},
  }

  cbx0_1 := func(cpu *Cpu) {
    reg := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
    result := reg.read()
    y := cpu.CurrentOpcode.Y
    var carry uint8
    switch y {
    case 0: // RLC
      carry = result >> 7
      result = (result << 1) | carry
    case 1: // RRC
      carry = result & 0x01
      result = (carry << 7) | (result >> 1)
    case 2: // RL
      carry = result >> 7
      result = (result << 1) | cpu.getFlagC()
    case 3: // RR
      carry = result & 0x1
      result = (cpu.getFlagC() << 7) | (result >> 1)
    case 4: // SLA
      carry = result >> 7
      result = result << 1
    case 5: // SRA
      carry = result & 0x01
      result = (result & 0b10000000) | (result >> 1)
    case 6: // SWAP
      result = ((result & 0x0F) << 4) | (result >> 4)
      carry = 0x0
    case 7: // SRL
      carry = result & 0x01
      result = result >> 1
    }

    if result == 0 {
      cpu.setFlagZ()
    } else {
      cpu.clearFlagZ()
    }

    cpu.clearFlagN()
    cpu.clearFlagH()

    if carry == 0x01 {
      cpu.setFlagC()
    } else {
      cpu.clearFlagC()
    }

    reg.write(result)
  }

  instructionMap["CBX0"] = Instruction{
    "rot[y] r[z]",
    1,
    []func(*Cpu){cbx0_1},
  }

  x0z7y2_1 := func(cpu *Cpu) {
    result := cpu.A.read()
    carry := result >> 7
    result = (result << 1) | cpu.getFlagC()

    cpu.clearFlagZ()
    cpu.clearFlagN()
    cpu.clearFlagH()
    if carry == 0x1 {
      cpu.clearFlagC()
    } else {
      cpu.setFlagC()
    }
  }

  instructionMap["X0Z7Y2"] = Instruction{
    "RLA",
    1,
    []func(*Cpu){x0z7y2_1},
  }

  x3z1q0_1 := func(cpu *Cpu) {
    // if reg is AF, it should automatically
    // set the flags, because they
    // are one and the same
    reg := cpu.rpTable[cpu.CurrentOpcode.P]
    val := cpu.Bus.memory.read(cpu.SP.read())
    reg.lo.write(val)
    cpu.SP.inc()
  }

  x3z1q0_2 := func(cpu *Cpu) {
    reg := cpu.rpTable[cpu.CurrentOpcode.P]
    val := cpu.Bus.memory.read(cpu.SP.read())
    reg.hi.write(val)
    cpu.SP.inc()
  }

  instructionMap["X3Z1Q0"] = Instruction{
    "POP rp2[p]",
    1,
    []func(*Cpu){x3z1q0_1, x3z1q0_2},
  }

  return instructionMap
}
