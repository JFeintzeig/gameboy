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
}

// TODO: document this logic and these magic numbers
func ByteToOpcode(oneByte uint8) Opcode {
  op := Opcode{
    Full: oneByte,
    X: uint8(0b11000000 & oneByte) >> 6,
    Y: uint8(0b111000 & oneByte) >> 3,
    Z: uint8(0b111 & oneByte),
    P: uint8(0b110000 & oneByte) >> 4,
    Q: (uint8(0b1000 & oneByte) >> 3) % 2,
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

  instructionMap ["X0Z1Q0"] = Instruction{
    "LD rp[p] nn",
    3,
    []func(*Cpu){x0z1q0_1, x0z1q0_2},
  }

  // X=2
  // uh oh, this will require a microop that
  // should take 0 cycles according to my current scheme
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
  instructionMap ["X2"] = Instruction{
    "alu[y] r[z]",
    1,
    []func(*Cpu,){x2_1},
  }

  return instructionMap
}
