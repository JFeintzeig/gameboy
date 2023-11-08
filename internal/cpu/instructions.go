package cpu

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

func (inst *Instruction) AddOpsToQueue(cpu *Cpu) {
  for _, op := range inst.operations {
    cpu.ExecutionQueue.Push(&op)
  }
}

// then the CPUs execution cycle should deal with timing and
// execute one microinstruction at a time
// Question: ...but then how does CPU call AddOpsToQueue() to
// populate the queue? and how deal with timing for the fetch
// instruction step? maybe after every instruction, a Fetch()
// function is added to the end of the queue to get the next opcode?
// then i guess a long case match statement to route incoming opcodes
// to instructions? wish the routing was easier...maybe if i give them
// all my own names then i could put them in a hashmap or something, maybe
// just like X1Y7Z2?

func MakeInstructionMap() map[string]Instruction {
  op1 := func (cpu *Cpu) {
    nn := cpu.ReadNN()
    cpu.rpTable[cpu.CurrentOpcode.P].writeLo(nn)
  }
  
  op2 := func (cpu *Cpu) {
    nn := cpu.ReadNN()
    cpu.rpTable[cpu.CurrentOpcode.P].writeHi(nn)
  }

  instructionMap := make(map[string]Instruction)

  // the keys in this map are just my internal
  // names based on the X/Y/Z/P/Q's we need to
  // match on, since its a many -> one mapping
  // of opcodes to actual exection instructions.
  instructionMap ["X0Z1Q0"] = Instruction{
    "LD rp[p] nn",
    3,
    []func(*Cpu,){op1, op2},
  }
  return instructionMap
}
