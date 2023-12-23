package cpu

func SetBit(value uint8, bitNum uint8, bitVal uint8) uint8 {
  if bitVal == 1 {
    return value | (bitVal << bitNum)
  } else {
    return value & ^(1 << bitNum)
  }
}

func GetBit(value uint8, bitNum uint8) uint8 {
  return (value >> bitNum) & 0x01
}

func SetBitBool(value uint8, bitNum uint8, bitVal bool) uint8 {
  if bitVal {
    return SetBit(value, bitNum, 1)
  } else {
    return SetBit(value, bitNum, 0)
  }
}

func GetBitBool(value uint8, bitNum uint8) bool {
    bitVal := GetBit(value, bitNum)
    if bitVal == 1 {
      return true
    } else {
      return false
    }
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
