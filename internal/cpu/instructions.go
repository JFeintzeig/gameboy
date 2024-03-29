package cpu

import "fmt"

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

func no_op(cpu *Cpu) {
  // just taking up time
  return
}

//TODO: this _should_ work but tetris still broken
// so for now i invalid logic with || !cpu.IncrementPC
// Maybe refactor so each instruction increments PC
// and CPU doesnt store any state about IncrementPC
// and FetchAndDecode just fetches without increments
func int_call_push_hi(cpu *Cpu) {
  cpu.SP.dec()
  var newPC uint16
  // TODO: bug here?
  newPC = cpu.PC.read()
  //if cpu.IncrementPC || !cpu.IncrementPC {
  //  newPC = cpu.PC.read() + uint16(cpu.OpcodeToInstruction(cpu.CurrentOpcode).nBytes)
  //} else {
  //  newPC = cpu.PC.read()
  //}
  cpu.Bus.WriteToBus(cpu.SP.read(), uint8(newPC >> 8))
}

//TODO: this _should_ work but tetris still broken
// so for now i invalid logic with || !cpu.IncrementPC
func int_call_push_lo(cpu *Cpu) {
  cpu.SP.dec()
  var newPC uint16
  // TODO: bug here?
  newPC = cpu.PC.read()
  //if cpu.IncrementPC || !cpu.IncrementPC {
  //  newPC = cpu.PC.read() + uint16(cpu.OpcodeToInstruction(cpu.CurrentOpcode).nBytes)
  //} else {
  //  newPC = cpu.PC.read()
  //}
  cpu.Bus.WriteToBus(cpu.SP.read(), uint8(newPC & 0xFF))
}

func call_push_hi(cpu *Cpu) {
  cpu.SP.dec()
  // return to _next_ instruction
  newPC := cpu.PC.read() + uint16(cpu.OpcodeToInstruction(cpu.CurrentOpcode).nBytes)
  cpu.Bus.WriteToBus(cpu.SP.read(), uint8(newPC >> 8))
}

func call_push_lo(cpu *Cpu) {
  cpu.SP.dec()
  // return to _next_ instruction
  newPC := cpu.PC.read() + uint16(cpu.OpcodeToInstruction(cpu.CurrentOpcode).nBytes)
  cpu.Bus.WriteToBus(cpu.SP.read(), uint8(newPC & 0xFF))
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

    // so confusing
    // turn to 4 bit numbers, add, see if 5th bit is set
    if (y == 0) && (((a & 0x0F) + (b & 0x0F)) & 0x10 == 0x10) {
      cpu.setFlagH()
    // same but with carry bit too. NB: do this before changing FlagC!
    } else if (y == 1) && (((a & 0x0F) + (b & 0x0F) + cpu.getFlagC()) & 0x10 == 0x10) {
      cpu.setFlagH()
    } else if (y == 2 || y == 7) && ((result & 0x0F) > (a & 0x0F)) {
      cpu.setFlagH()
    } else if (y == 3) && (((a & 0x0F) - (b & 0x0F) - cpu.getFlagC()) & 0x10 == 0x10) {
      cpu.setFlagH()
    } else if y == 4 {
      // really?
      cpu.setFlagH()
    } else {
      cpu.clearFlagH()
    }

    if (y == 0) && (result < a || result < b) {
      cpu.setFlagC()
    } else if (y == 1) && (result < a + cpu.getFlagC() || result < b + cpu.getFlagC() || a + b < a) {
      cpu.setFlagC()
    } else if (y == 2 || y == 7) && (result > a) {
      cpu.setFlagC()
    // need to also check for if b + carry _overflows_ :(
    } else if (y == 3) && (b > a || cpu.getFlagC() > a || b + cpu.getFlagC() > a || b + cpu.getFlagC() < b) {
      cpu.setFlagC()
    } else {
      cpu.clearFlagC()
    }

    // store result
    if y != 7 {
      cpu.A.write(result)
    }
    return
  }

// TODO: go through and re-check timing for each instr
// TODO: and add no_ops to one's where previously FetchAndDecode()
// TODO: was taking up a cycle
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
    cpu.PC.inc()
    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X0Z1Q0"] = Instruction{
    "LD rp[p] nn",
    3,
    []func(*Cpu){no_op, x0z1q0_1, x0z1q0_2},
  }

  // X=0, Z=2, P=3, Q=0
  x0z2q0p3_1 := func (cpu *Cpu) {
    cpu.Bus.WriteToBus(cpu.HL.read(), cpu.A.read())
    cpu.HL.dec()
    cpu.PC.inc()
  }

  instructionMap["X0Z2P3Q0"] = Instruction {
   "LDD (HL) A",
   1,
   []func(*Cpu){x0z2q0p3_1, no_op},
  }

  // X=0, Z=2, P=2, Q=0
  x0z2q0p2_1 := func (cpu *Cpu) {
    cpu.Bus.WriteToBus(cpu.HL.read(), cpu.A.read())
    cpu.HL.inc()
    cpu.PC.inc()
  }

  instructionMap["X0Z2P2Q0"] = Instruction {
   "LDI (HL) A",
   1,
   []func(*Cpu){x0z2q0p2_1, no_op},
  }

  // X=0, Z=2, P=3, Q=1
  x0z2q1p3_1 := func (cpu *Cpu) {
    cpu.A.write(cpu.Bus.ReadFromBus(cpu.HL.read()))
    cpu.HL.dec()
    cpu.PC.inc()
  }

  instructionMap["X0Z2P3Q1"] = Instruction {
   "LDD A (HL)",
   1,
   []func(*Cpu){x0z2q1p3_1, no_op},
  }

  // X=0, Z=2, P=2, Q=1
  x0z2q1p2_1 := func (cpu *Cpu) {
    cpu.A.write(cpu.Bus.ReadFromBus(cpu.HL.read()))
    cpu.HL.inc()
    cpu.PC.inc()
  }

  instructionMap["X0Z2P2Q1"] = Instruction {
   "LDI A (HL)",
   1,
   []func(*Cpu){x0z2q1p2_1, no_op},
  }

  // X=2
  x2_1 := func (cpu *Cpu) {
    a := cpu.A.read()
    var b uint8
    if (cpu.CurrentOpcode.Z != 6) {
      register := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
      b = register.read()
    } else {
      b = cpu.Bus.ReadFromBus(cpu.HL.read())
    }

    cpu.DoAluInstruction(a, b)
    if cpu.CurrentOpcode.Z == 6 {
      cpu.ExecutionQueue.Push(no_op)
    }
    cpu.PC.inc()
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

  x3y7z3 := func (cpu *Cpu) {
    // EI: set IME flag _after_ the following instruction
    // - we set IMECountdown to 1
    // - IMECountdown is decremented each time an instruction is fetched until it reaches 0
    cpu.IMECountdown = 1
    cpu.PC.inc()
  }

  instructionMap["X3Y7Z3"] = Instruction{
    "EI",
    1,
    []func(*Cpu){x3y7z3},
  }

  x0z6_1 := func (cpu *Cpu) {
    if cpu.CurrentOpcode.Y != 6 {
      cpu.GetRTableRegister(cpu.CurrentOpcode.Y).write(cpu.ReadN())
    } else if cpu.CurrentOpcode.Y == 6 {
      cpu.Bus.WriteToBus(cpu.HL.read(), cpu.ReadN())
      cpu.ExecutionQueue.Push(no_op)
    }
    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X0Z6"] = Instruction{
    "LD r[y], N",
    2,
    []func(*Cpu){no_op, x0z6_1},
  }

  x3z2y4_1 := func (cpu *Cpu) {
    cpu.Bus.WriteToBus(0xFF00 + uint16(cpu.C.read()), cpu.A.read())
    cpu.PC.inc()
  }

  instructionMap["X3Z2Y4"] = Instruction{
    "LD [0xFF00 + C], A",
    1,
    []func(*Cpu){x3z2y4_1, no_op},
  }

  x3z2y6_1 := func (cpu *Cpu) {
    cpu.A.write(cpu.Bus.ReadFromBus(0xFF00 + uint16(cpu.C.read())))
    cpu.PC.inc()
  }

  instructionMap["X3Z2Y6"] = Instruction{
    "LD A, [0xFF00 + C]",
    1,
    []func(*Cpu){x3z2y6_1, no_op},
  }

  x3y6z3_1 := func (cpu *Cpu) {
    cpu.IME = false
    cpu.PC.inc()
  }

  instructionMap["X3Y6Z3"] = Instruction{
    "DI",
    1,
    []func(*Cpu){x3y6z3_1},
  }

  x1_1:= func (cpu *Cpu) {
    if cpu.CurrentOpcode.Y != 6 && cpu.CurrentOpcode.Z != 6 {
      from := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
      to := cpu.GetRTableRegister(cpu.CurrentOpcode.Y)
      to.write(from.read())
    } else if cpu.CurrentOpcode.Y == 6 {
      from := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
      cpu.Bus.WriteToBus(cpu.HL.read(), from.read())

      // ideally this would happen _before_ x1_1 is executed
      cpu.ExecutionQueue.Push(no_op)
    } else if cpu.CurrentOpcode.Z == 6 {
      to := cpu.GetRTableRegister(cpu.CurrentOpcode.Y)
      to.write(cpu.Bus.ReadFromBus(cpu.HL.read()))

      cpu.ExecutionQueue.Push(no_op)
    }
    cpu.PC.inc()
  }

  instructionMap["X1"] = Instruction{
    "LD r[y] r[z]",
    1,
    []func(*Cpu){x1_1},
  }

  x3z0y4_2 := func (cpu *Cpu) {
    n := cpu.ReadN()
    cpu.Bus.WriteToBus(0xFF00 + uint16(n), cpu.A.read())
    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X3Z0Y4"] = Instruction{
    "LD [0xFF00+u8], A",
    2,
    []func(*Cpu){no_op, x3z0y4_2, no_op},
  }

  x3z5q0_2 := func (cpu *Cpu) {
    cpu.SP.dec()
    cpu.Bus.WriteToBus(cpu.SP.read(), cpu.rp2Table[cpu.CurrentOpcode.P].readHi())
  }

  x3z5q0_3 := func (cpu *Cpu) {
    cpu.SP.dec()
    cpu.Bus.WriteToBus(cpu.SP.read(), cpu.rp2Table[cpu.CurrentOpcode.P].readLo())
    cpu.PC.inc()
  }

  instructionMap["X3Z5Q0"] = Instruction{
    "PUSH rp2[p]",
    1,
    []func(*Cpu){no_op, no_op,x3z5q0_2,x3z5q0_3},
  }

  x0z2p0q0_1 := func (cpu *Cpu) {
    cpu.Bus.WriteToBus(cpu.BC.read(), cpu.A.read())
    cpu.PC.inc()
  }

  instructionMap["X0Z2P0Q0"] = Instruction{
    "LD [BC], A",
    1,
    []func(*Cpu){x0z2p0q0_1, no_op},
  }

  x0z2p1q0_1 := func (cpu *Cpu) {
    cpu.Bus.WriteToBus(cpu.DE.read(), cpu.A.read())
    cpu.PC.inc()
  }

  instructionMap["X0Z2P1Q0"] = Instruction{
    "LD [DE], A",
    1,
    []func(*Cpu){x0z2p1q0_1, no_op},
  }

  x0z2p1q1_1 := func (cpu *Cpu) {
    cpu.A.write(cpu.Bus.ReadFromBus(cpu.DE.read()))
    cpu.PC.inc()
  }

  instructionMap["X0Z2P1Q1"] = Instruction{
    "LD A, [DE]",
    1,
    []func(*Cpu){x0z2p1q1_1, no_op},
  }

  x0z2p0q1_1 := func (cpu *Cpu) {
    cpu.A.write(cpu.Bus.ReadFromBus(cpu.BC.read()))
    cpu.PC.inc()
  }

  instructionMap["X0Z2P0Q1"] = Instruction{
    "LD A, [BC]",
    1,
    []func(*Cpu){x0z2p0q1_1, no_op},
  }

  // combining Q=0 and Q=1 into one function
  x0z3_1 := func (cpu *Cpu) {
    reg := cpu.rpTable[cpu.CurrentOpcode.P]
    if cpu.CurrentOpcode.Q == 0 {
      reg.inc()
    } else if cpu.CurrentOpcode.Q == 1 {
      reg.dec()
    }
    cpu.PC.inc()
  }

  instructionMap["X0Z3"] = Instruction{
    "INC/DEC rp[p]",
    1,
    []func(*Cpu){no_op, x0z3_1},
  }

  x0z0ygte4_1 := func (cpu *Cpu) {
      x0z0ygte4_2 := func (cpu *Cpu) {
        // NB: the relative jump is relative to the
        // instruction _after_ this one.
        newPC := cpu.PC.read() + uint16(cpu.ReadD()) + 2
        cpu.PC.write(newPC)
      }

    cond := cpu.GetCCTableBool(cpu.CurrentOpcode.Y-4)
    if (cond) {
      // will this break shit? def. feels like
      // crossing an encapsulation boundary at least
      cpu.ExecutionQueue.Push(x0z0ygte4_2)
    } else {
      cpu.PC.inc()
      cpu.PC.inc()
      return
    }
  }

  instructionMap["X0Z0Ygte4"] = Instruction{
    "JR cc[y-4], d",
    2,
    []func(*Cpu){no_op, x0z0ygte4_1},
  }

  x0z0y3_1 := func (cpu *Cpu) {
    // NB: the relative jump is relative to the
    // instruction _after_ this one.
    newPC := cpu.PC.read() + uint16(cpu.ReadD()) + 2
    cpu.PC.write(newPC)
  }

  instructionMap["X0Z0Y3"] = Instruction{
    "JR d",
    2,
    []func(*Cpu){no_op, no_op, x0z0y3_1},
  }

  x3z6_1 := func (cpu *Cpu) {
    a := cpu.A.read()
    b := cpu.ReadN()
    cpu.DoAluInstruction(a, b)
    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X3Z6"] = Instruction{
    "alu[y] n",
    2,
    []func(*Cpu){no_op, x3z6_1},
  }

  call_push_lo_and_jump := func (cpu *Cpu) {
    call_push_lo(cpu)
    if cpu.CurrentOpcode.Z == 7 {
      // RST
      cpu.PC.write(uint16(cpu.CurrentOpcode.Y*8))
    } else {
      // CALL
      cpu.PC.write(cpu.ReadNN())
    }
  }

  instructionMap["X3Z5P0Q1"] = Instruction{
    "CALL NN",
    3,
    []func(*Cpu){no_op, no_op, no_op, no_op, call_push_hi, call_push_lo_and_jump},
  }

  x3z4ylte3_branch := func (cpu *Cpu) {
    cond := cpu.GetCCTableBool(cpu.CurrentOpcode.Y)
    if cond {
      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(call_push_hi)
      cpu.ExecutionQueue.Push(call_push_lo_and_jump)
    } else {
      cpu.PC.inc()
      cpu.PC.inc()
      cpu.PC.inc()
    }
  }

  instructionMap["X3Z4Ylte3"] = Instruction{
    "CALL cc[y] NN",
    3,
    []func(*Cpu){no_op, no_op, x3z4ylte3_branch},
  }

  instructionMap["X3Z7"] = Instruction{
    "RST y*8",
    1,
    []func(*Cpu){no_op, no_op, call_push_hi, call_push_lo_and_jump},
  }

  x3z0y6_2 := func(cpu *Cpu) {
    cpu.A.write(cpu.Bus.ReadFromBus(0xFF00 + uint16(cpu.ReadN())))
    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X3Z0Y6"] = Instruction{
    "LD A, [0xFF00+n]",
    2,
    []func(*Cpu){no_op, x3z0y6_2, no_op},
  }

  x0z5_1 := func(cpu *Cpu) {
    var val uint8
    if cpu.CurrentOpcode.Y != 6 {
      reg := cpu.GetRTableRegister(cpu.CurrentOpcode.Y)
      reg.dec()
      val = reg.read()
    } else {
      memAtHL := cpu.Bus.ReadFromBus(cpu.HL.read())
      val = memAtHL - 1
      cpu.Bus.WriteToBus(cpu.HL.read(), val)

      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(no_op)
    }

    if val == 0 {
      cpu.setFlagZ()
    } else {
      cpu.clearFlagZ()
    }

    cpu.setFlagN()

    if ((val & 0x0F) > ((val+1) & 0x0F)) {
      cpu.setFlagH()
    } else {
      cpu.clearFlagH()
    }
    cpu.PC.inc()
  }

  instructionMap["X0Z5"] = Instruction{
    "DEC r[y]",
    1,
    []func(*Cpu){x0z5_1},
  }

  x0z4_1 := func(cpu *Cpu) {
    var val uint8
    if cpu.CurrentOpcode.Y != 6 {
      reg := cpu.GetRTableRegister(cpu.CurrentOpcode.Y)
      reg.inc()
      val = reg.read()
    } else {
      memAtHL := cpu.Bus.ReadFromBus(cpu.HL.read())
      val = memAtHL + 1
      cpu.Bus.WriteToBus(cpu.HL.read(), val)
      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(no_op)
    }

    if val == 0 {
      cpu.setFlagZ()
    } else {
      cpu.clearFlagZ()
    }

    cpu.clearFlagN()

    if ((val & 0x0F) < ((val-1) & 0x0F)) {
      cpu.setFlagH()
    } else {
      cpu.clearFlagH()
    }
    cpu.PC.inc()
  }

  instructionMap["X0Z4"] = Instruction{
    "INC r[y]",
    1,
    []func(*Cpu){x0z4_1},
  }

  ret := func(cpu *Cpu) {
    lower := cpu.Bus.ReadFromBus(cpu.SP.read())
    cpu.SP.inc()
    upper := cpu.Bus.ReadFromBus(cpu.SP.read())
    cpu.SP.inc()

    cpu.PC.write(uint16(upper) << 8 | uint16(lower))
  }

  instructionMap["X3Z1Q1P0"] = Instruction{
    "RET",
    1,
    []func(*Cpu){no_op, no_op, no_op, ret},
  }

  x3z1q1p1_1 := func(cpu *Cpu) {
    // Set IME immediately after this instruction
    cpu.IMECountdown = 0
    ret(cpu)
  }

  // for this and RET, timing of writes don't
  // line up exactly
  instructionMap["X3Z1Q1P1"] = Instruction{
    "RETI",
    1,
    []func(*Cpu){no_op, no_op, no_op, x3z1q1p1_1},
  }

  x3z0ylte3_1 := func(cpu *Cpu) {
    cond := cpu.GetCCTableBool(cpu.CurrentOpcode.Y)
    if (cond) {
      // will this break shit? def. feels like
      // crossing an encapsulation boundary at least
      //fmt.Printf("RET CC[%d] happening\n", cpu.CurrentOpcode.Y)
      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(ret)
    } else {
      cpu.PC.inc()
    }
  }

  instructionMap["X3Z0Ylte3"] = Instruction{
    "RET cc[y]",
    1,
    []func(*Cpu){no_op, x3z0ylte3_1},
  }

  no_op_inc_pc := func(cpu *Cpu) {
    cpu.PC.inc()
  }

  instructionMap["X0Z0Y0"] = Instruction{
    "NOP",
    1,
    []func(*Cpu){no_op_inc_pc},
  }

  x3z3y0_1 := func(cpu *Cpu) {
    nn := cpu.ReadNN()
    cpu.PC.write(nn)
  }

  instructionMap["X3Z3Y0"] = Instruction{
    "JP NN",
    3,
    []func(*Cpu){no_op, no_op, no_op, x3z3y0_1},
  }

  x3z1q1p2_1 := func(cpu *Cpu) {
    cpu.PC.write(cpu.HL.read())
  }

  instructionMap["X3Z1Q1P2"] = Instruction{
    "JP HL",
    1,
    []func(*Cpu){x3z1q1p2_1},
  }

  x3z2ylte3_1 := func (cpu *Cpu) {
      // function to do the jump
      x3z2ylte3_2 := func (cpu *Cpu) {
        // TODO: does this work?? signed and unsigned
        // confusion
        newPC := cpu.ReadNN()
        cpu.PC.write(newPC)
      }

    cond := cpu.GetCCTableBool(cpu.CurrentOpcode.Y)
    if (cond) {
      // will this break shit? def. feels like
      // crossing an encapsulation boundary at least
      cpu.ExecutionQueue.Push(x3z2ylte3_2)
    } else {
      cpu.PC.inc()
      cpu.PC.inc()
      cpu.PC.inc()
    }
  }

  instructionMap["X3Z2Ylte3"] = Instruction{
    "JP cc[y], nn",
    3,
    []func(*Cpu){no_op, no_op, x3z2ylte3_1},
  }

  x3z1q0_1 := func(cpu *Cpu) {
    // if reg is AF, it should automatically
    // set the flags, because they
    // are one and the same
    reg := cpu.rp2Table[cpu.CurrentOpcode.P]
    val := cpu.Bus.ReadFromBus(cpu.SP.read())
    reg.lo.write(val)
    // AF: zero-out bottom nibble
    if cpu.CurrentOpcode.P == 3 {
      reg.lo.write(reg.lo.read() & 0xF0)
    }
    cpu.SP.inc()
  }

  x3z1q0_2 := func(cpu *Cpu) {
    reg := cpu.rp2Table[cpu.CurrentOpcode.P]
    val := cpu.Bus.ReadFromBus(cpu.SP.read())
    reg.hi.write(val)
    cpu.SP.inc()
    cpu.PC.inc()
  }

  instructionMap["X3Z1Q0"] = Instruction{
    "POP rp2[p]",
    1,
    []func(*Cpu){no_op, x3z1q0_1, x3z1q0_2},
  }

  x3z2y7_1 := func(cpu *Cpu) {
    cpu.A.write(cpu.Bus.ReadFromBus(cpu.ReadNN()))
    cpu.PC.inc()
    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X3Z2Y7"] = Instruction{
    "LD A, [NN]",
    3,
    []func(*Cpu){no_op, no_op, x3z2y7_1, no_op},
  }

  x3z2y5_1 := func(cpu *Cpu) {
    cpu.Bus.WriteToBus(cpu.ReadNN(), cpu.A.read())
    cpu.PC.inc()
    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X3Z2Y5"] = Instruction{
    "LD [NN], A",
    3,
    []func(*Cpu){no_op, no_op, x3z2y5_1, no_op},
  }

  // same as the rot functions but
  // just for A and always clear Z
  x0z7ylte3_1 := func(cpu *Cpu) {
    reg := &cpu.A
    result := (*reg).read()
    y := cpu.CurrentOpcode.Y
    var carry uint8
    switch y {
    case 0: // RLCA
      carry = result >> 7
      result = (result << 1) | carry
    case 1: // RRCA
      carry = result & 0x01
      result = (carry << 7) | (result >> 1)
    case 2: // RLA
      carry = result >> 7
      result = (result << 1) | cpu.getFlagC()
    case 3: // RRA
      carry = result & 0x1
      result = (cpu.getFlagC() << 7) | (result >> 1)
    }

    if carry == 0x01 {
      cpu.setFlagC()
    } else {
      cpu.clearFlagC()
    }
    cpu.clearFlagZ()
    cpu.clearFlagN()
    cpu.clearFlagH()

    (*reg).write(result)
    cpu.PC.inc()
  }

  instructionMap["X0Z7Ylte3"] = Instruction{
    "RLCA/RRCA/RLA/RRA",
    1,
    []func(*Cpu){x0z7ylte3_1},
  }

  x0z7y5_1 := func(cpu *Cpu) {
    cpu.A.write(^cpu.A.read())
    cpu.setFlagN()
    cpu.setFlagH()
    cpu.PC.inc()
  }

  instructionMap["X0Z7Y5"] = Instruction{
    "CPL",
    1,
    []func(*Cpu){x0z7y5_1},
  }

  x0z7y6_1 := func(cpu *Cpu) {
    cpu.setFlagC()
    cpu.clearFlagN()
    cpu.clearFlagH()
    cpu.PC.inc()
  }

  instructionMap["X0Z7Y6"] = Instruction{
    "SCF",
    1,
    []func(*Cpu){x0z7y6_1},
  }

  x0z7y7_1 := func(cpu *Cpu) {
    if cpu.getFlagC() == 0 {
      cpu.setFlagC()
    } else {
      cpu.clearFlagC()
    }
    cpu.clearFlagN()
    cpu.clearFlagH()
    cpu.PC.inc()
  }

  instructionMap["X0Z7Y7"] = Instruction{
    "CCF",
    1,
    []func(*Cpu){x0z7y7_1},
  }

  x0z1q1_1 := func(cpu *Cpu) {
    a := cpu.HL.read()
    b := cpu.rpTable[cpu.CurrentOpcode.P].read()
    result := a + b

    if result < a {
      cpu.setFlagC()
    } else {
      cpu.clearFlagC()
    }

    if (result & 0xFFF) < (a & 0xFFF) {
      cpu.setFlagH()
    } else {
      cpu.clearFlagH()
    }
    cpu.clearFlagN()

    cpu.HL.write(result)
    cpu.PC.inc()
  }

  instructionMap["X0Z1Q1"] = Instruction{
    "ADD HL, rp[p]",
    1,
    []func(*Cpu){no_op, x0z1q1_1},
  }

  x0z0y1_1 := func(cpu *Cpu) {
    cpu.Bus.WriteToBus(cpu.ReadNN(), cpu.SP.readLo())
  }

  x0z0y1_2 := func(cpu *Cpu) {
    cpu.Bus.WriteToBus(cpu.ReadNN() + 1, cpu.SP.readHi())
    cpu.PC.inc()
    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X0Z0Y1"] = Instruction{
    "LD (nn) SP",
    3,
    []func(*Cpu){no_op, no_op, no_op, x0z0y1_1, x0z0y1_2},
  }

  x0z0y2_1 := func(cpu *Cpu) {
    fmt.Printf("STOP PC %04X SP %04X GC %d\n", cpu.PC.read(), cpu.SP.read(), cpu.globalCounter)
    panic("STOP")
  }

  instructionMap["X0Z0Y2"] = Instruction{
    "STOP",
    1,
    []func(*Cpu){x0z0y2_1},
  }

  x3z1q1p3_1 := func(cpu *Cpu) {
    cpu.SP.write(cpu.HL.read())
    cpu.PC.inc()
  }

  instructionMap["X3Z1Q1P3"] = Instruction{
    "LD SP, HL",
    1,
    []func(*Cpu){no_op, x3z1q1p3_1},
  }

  x3z0y5_1 := func(cpu *Cpu) {
    // TODO: D is signed, does this mess it up?
    oldSP := cpu.SP.read()
    d := uint16(cpu.ReadD())
    cpu.SP.write(oldSP + d)
    newSPLo := cpu.SP.readLo()

    cpu.clearFlagZ()
    cpu.clearFlagN()

    if (newSPLo < uint8(d) || newSPLo < uint8(oldSP & 0xFF)) {
      cpu.setFlagC()
    } else {
      cpu.clearFlagC()
    }

    if (newSPLo & 0x0F) < (uint8(d) & 0x0F) {
      cpu.setFlagH()
    } else {
      cpu.clearFlagH()
    }

    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X3Z0Y5"] = Instruction{
    "ADD SP, d",
    2,
    []func(*Cpu){no_op, no_op, no_op, x3z0y5_1},
  }

  x3z0y7_1 := func(cpu *Cpu) {
    // TODO: D is signed, does this mess it up?
    sp := cpu.SP.read()
    d := uint16(cpu.ReadD())
    result := sp + d
    cpu.HL.write(result)

    cpu.clearFlagZ()
    cpu.clearFlagN()
    if (result & 0xFF) < (sp & 0xFF) || (result & 0xFF) < (d & 0xFF) {
      cpu.setFlagC()
    } else {
      cpu.clearFlagC()
    }
    if (result & 0x0F) < (sp & 0x0F) || (result & 0x0F) < (d & 0x0F) {
      cpu.setFlagH()
    } else {
      cpu.clearFlagH()
    }

    cpu.PC.inc()
    cpu.PC.inc()
  }

  instructionMap["X3Z0Y7"] = Instruction{
    "LD HL, SP + d",
    2,
    []func(*Cpu){no_op, no_op, x3z0y7_1},
  }

  // https://blog.ollien.com/posts/gb-daa/
  x0z7y4_1 := func(cpu *Cpu) {
    a := cpu.A.read()
    halfCarry := false
    if cpu.getFlagN() == 0 {
      if (a&0x0F) > 0x09 {
        a += 0x06
        halfCarry = true
      } else if cpu.getFlagH() == 1 {
        // we are not overflowing bottom digit here
        // so don't set halfCarry
        a += 0x06
      }

      // if bottom digit carries and top digit is 0, that
      // means we overflowed the whole thing
      // also if C was set, we already overflowed so need to add 0x60
      if ((a & 0xF0) > 0x90) || ((a & 0xF0) == 0 && halfCarry) || cpu.getFlagC() == 1 {
        a += 0x60
        cpu.setFlagC()
      } else {
        cpu.clearFlagC()
      }
    } else {
      if cpu.getFlagC() == 1 {
        a -= 0x60
      }
      if cpu.getFlagH() == 1 {
        a -= 0x06
      }
    }

    cpu.clearFlagH()
    if a == 0 {
      cpu.setFlagZ()
    } else {
      cpu.clearFlagZ()
    }

    cpu.A.write(a)
    cpu.PC.inc()
  }

  instructionMap["X0Z7Y4"] = Instruction{
    "DAA",
    1,
    []func(*Cpu){x0z7y4_1},
  }

  // CB instructions
  cbx0_1 := func(cpu *Cpu) {
    var result uint8
    if cpu.CurrentOpcode.Z != 6 {
      result = cpu.GetRTableRegister(cpu.CurrentOpcode.Z).read()
    } else {
      result = cpu.Bus.ReadFromBus(cpu.HL.read())
    }

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

    if cpu.CurrentOpcode.Z != 6 {
      cpu.GetRTableRegister(cpu.CurrentOpcode.Z).write(result)
    } else {
      cpu.Bus.WriteToBus(cpu.HL.read(), result)

      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(no_op)
    }

    cpu.PC.inc()
  }

  instructionMap["CBX0"] = Instruction{
    "rot[y] r[z]",
    1,
    []func(*Cpu){no_op, cbx0_1},
  }

  cbx1_1 := func (cpu *Cpu) {
    oc := cpu.CurrentOpcode
    var value uint8
    if oc.Z != 6 {
      value = cpu.GetRTableRegister(cpu.CurrentOpcode.Z).read()
    } else {
      value = cpu.Bus.ReadFromBus(cpu.HL.read())
    }

    //TODO: replace with Get/Set bit util functions; same with flags
    result := (value >> oc.Y) & 0x01
    if result == 0 {
      cpu.setFlagZ()
    } else {
      cpu.clearFlagZ()
    }
    cpu.clearFlagN()
    cpu.setFlagH()

    if cpu.CurrentOpcode.Z == 6 {
      cpu.ExecutionQueue.Push(no_op)
    }

    cpu.PC.inc()
  }

  instructionMap["CBX1"] = Instruction{
    "BIT y, r[z]",
    1,
    []func(*Cpu){no_op, cbx1_1},
  }

  cbx2_2 := func(cpu *Cpu) {
    y := cpu.CurrentOpcode.Y
    value := cpu.Bus.ReadFromBus(cpu.HL.read())
    result := value & ^(0x1 << y)
    cpu.Bus.WriteToBus(cpu.HL.read(), result)
  }

  cbx2_1 := func(cpu *Cpu) {
    y := cpu.CurrentOpcode.Y
    if cpu.CurrentOpcode.Z != 6 {
      reg := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
      // make a mask with all 0's and a single 1 in the yth
      // place, then take complement
      reg.write(reg.read() & ^(0x1 << y))
    } else {
      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(cbx2_2)
    }

    cpu.PC.inc()
  }

  instructionMap["CBX2"] = Instruction{
    "RES y, r[z]",
    1,
    []func(*Cpu){no_op, cbx2_1},
  }

  cbx3_2 := func(cpu *Cpu) {
    y := cpu.CurrentOpcode.Y
    value := cpu.Bus.ReadFromBus(cpu.HL.read())
    result := (value | (0x1 << y))
    cpu.Bus.WriteToBus(cpu.HL.read(), result)
  }

  cbx3_1 := func(cpu *Cpu) {
    y := cpu.CurrentOpcode.Y
    if cpu.CurrentOpcode.Z != 6 {
      reg := cpu.GetRTableRegister(cpu.CurrentOpcode.Z)
      // make a mask with all 0's and a single 1 in the yth
      // place, then OR it with reg
      reg.write(reg.read() | (0x1 << y))
    } else {
      cpu.ExecutionQueue.Push(no_op)
      cpu.ExecutionQueue.Push(cbx3_2)
    }

    cpu.PC.inc()
  }

  instructionMap["CBX3"] = Instruction{
    "SET y, r[z]",
    1,
    []func(*Cpu){no_op, cbx3_1},
  }

  // TODO: implement HALT bug
  halt := func(cpu *Cpu){
    // make it halt
    if !cpu.isHalted {
      //fmt.Printf("starting halt PC:%04X IME:%t IE:%08b IF:%08b GC:%d\n", cpu.PC.read(), cpu.IME, cpu.Bus.ReadFromBus(IE), cpu.Bus.ReadFromBus(IF), cpu.globalCounter)
    }
    cpu.isHalted = true

    pendingInt := (cpu.Bus.ReadFromBus(IE) & cpu.Bus.ReadFromBus(IF)) != 0

    // unhalt if pending interrupt, regardless of IME
    // DoInterrupts() will check IME to decide whether to service interrupt
    if pendingInt || cpu.justDidInterrupt {
      cpu.isHalted = false
      cpu.PC.inc()
      //fmt.Printf("unhalted PC:%04X GC:%d\n", cpu.PC.read(), cpu.globalCounter)
      return
    }

    // if no pending interrupt, keep halting
    return
  }

  instructionMap["X1Z6Y6"] = Instruction{
    "HALT",
    1,
    []func(*Cpu){halt},
  }

  return instructionMap
}
