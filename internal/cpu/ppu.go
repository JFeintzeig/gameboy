package cpu

import (
  //"fmt"
)

type Mode int

const N_MODES = 4

// where to store metadata associated w/Mode (total # dots,
// but it varies) and FetcherState (# of dots)
const (
  M0 Mode = iota
  M1
  M2
  M3
)

type FetcherState int

const N_FETCHER_STATES = 4

const (
  GetTile FetcherState = iota
  GetTileDataLow 
  GetTileDataHigh
  Push
)

func (fs *FetcherState) next() FetcherState {
  return (*fs + GetTileDataLow) % N_FETCHER_STATES
}

type Pixel struct {
  color uint8
  palette uint8
  bgPriority uint8
}

func NewPpu(busPointer *Bus) *Ppu {
  ppu := Ppu{}

  ppu.currentMode = M2
  ppu.LY.write(0)
  ppu.currentFetcherState = 0

  ppu.bus = busPointer
  ppu.vram = [8*1024]uint8{}
  ppu.screen = [160*144]uint8{}

  ppu.applyFetcherState = [N_FETCHER_STATES]func()bool{
    ppu.GetTile,
    ppu.GetTileDataLow,
    ppu.GetTileDataHigh,
    ppu.Push,
  }

  return &ppu
}

type Ppu struct {
  bus Mediator

  vram [8*1024]uint8
  screen [160*144]uint8

  nDots uint16
  currentFetcherState FetcherState
  fetcherX uint8
  fetcherY uint8

  applyFetcherState [4]func() bool

  bgFifo Fifo[Pixel]
  objFifo Fifo[Pixel]

  // LCDC
  lcdEnable bool
  windowMapAddress bool
  windowEnable bool
  bgWinDataAddress bool
  bgMapAddress bool
  objSize bool
  objEnable bool
  bgWinDisplay bool

  LY Register8
  LYC Register8

  // STAT
  lycInt bool
  mode2Int bool
  mode1Int bool
  mode0Int bool
  LYCeqLY bool
  currentMode Mode
}

func (ppu *Ppu) GetTile() bool {
  return true
}

func (ppu *Ppu) GetTileDataLow() bool {
  return true
}

func (ppu *Ppu) GetTileDataHigh() bool {
  return true
}

// TODO: control-flow, how to make this
// "attempted every dot until it succeeds"?
func (ppu *Ppu) Push() bool {
  success := false
  return success
}

func (ppu *Ppu) doFetchRoutine() {
    success := ppu.applyFetcherState[ppu.currentFetcherState]()

    if ppu.currentFetcherState == Push && !success {
      return
    }

    ppu.currentFetcherState = ppu.currentFetcherState.next()
}

func (ppu *Ppu) doCycle() {
  //fmt.Printf("NDOTS: %d MODE: %d LY: %d FETCHER: %d\n", ppu.nDots, ppu.currentMode, ppu.LY.read(), ppu.currentFetcherState)

  ppu.nDots += 4

  if ppu.LYC == ppu.LY {
    ppu.LYCeqLY = true
    // fire interrupt
  } else {
    ppu.LYCeqLY = false
  }

  if ppu.currentMode == M2 {
    // implement M2

    if ppu.nDots == 80 {
      ppu.nDots = 0
      ppu.currentMode = M3
      // fire interrupt
    }
  } else if ppu.currentMode == M3 {
    // 4 dots worth
    ppu.doFetchRoutine()
    ppu.doFetchRoutine()

    if ppu.nDots == 172 { // TODO: penalties
      ppu.currentMode = M0
      ppu.currentFetcherState = 0
      // don't reset nDots here, keep counting to end of line
      // fire interrupt
    }
  } else if ppu.currentMode == M0 {
    // HBlank - do stuff

    // at end of routine, if 
    if ppu.nDots == 376 {
      ppu.LY.inc()
      ppu.nDots = 0
      if ppu.LY.read() == 144 {
        ppu.currentMode = M1
      } else {
        ppu.currentMode = M2
      }
    }
  } else if ppu.currentMode == M1 {
    if ppu.nDots == 4560 {
      // end of VBlank, move to M2 + new frame
      ppu.nDots = 0
      ppu.LY.write(0)
      ppu.currentMode = M2

      // fire interrupt?
    } else if ppu.nDots % 456 == 0 {
      // end of scanline
      ppu.LY.inc()
    }
  }
}

func (ppu *Ppu) readVRAM(address uint16) uint8 {
  // TODO
  return 0xFF
}

func (ppu *Ppu) writeVRAM(address uint16, value uint8) {
  // TODO
}

func (ppu *Ppu) readRegister(address uint16) uint8 {
  if address == 0xFF40 {
    var result uint8
    result = SetBitBool(result, 7, ppu.lcdEnable)
    result = SetBitBool(result, 6, ppu.windowMapAddress)
    result = SetBitBool(result, 5, ppu.windowEnable)
    result = SetBitBool(result, 4, ppu.bgWinDataAddress)
    result = SetBitBool(result, 3, ppu.bgMapAddress)
    result = SetBitBool(result, 2, ppu.objSize)
    result = SetBitBool(result, 1, ppu.objEnable)
    result = SetBitBool(result, 0, ppu.bgWinDisplay)
    return result
  } else if address == 0xFF41 {
    var result uint8
    result = SetBitBool(result, 6, ppu.lycInt)
    result = SetBitBool(result, 5, ppu.mode2Int)
    result = SetBitBool(result, 4, ppu.mode1Int)
    result = SetBitBool(result, 3, ppu.mode0Int)
    result = SetBitBool(result, 2, ppu.LYCeqLY)
    result |= uint8(ppu.currentMode)
    return result
  } else if address == 0xFF44 {
    return ppu.LY.read()
  } else if address == 0xFF45 {
    return ppu.LYC.read()
  }
  // TODO
  return 0xFF
}

func (ppu *Ppu) writeRegister(address uint16, value uint8) {
  if address == 0xFF40 {
    ppu.lcdEnable = GetBitBool(value, 7)
    ppu.windowMapAddress = GetBitBool(value, 6)
    ppu.windowEnable = GetBitBool(value, 5)
    ppu.bgWinDataAddress = GetBitBool(value, 4)
    ppu.bgMapAddress = GetBitBool(value, 3)
    ppu.objSize = GetBitBool(value, 2)
    ppu.objEnable = GetBitBool(value, 1)
    ppu.bgWinDisplay = GetBitBool(value, 0)
  } else if address == 0xFF41 {
    ppu.lycInt = GetBitBool(value,6)
    ppu.mode2Int = GetBitBool(value,5)
    ppu.mode1Int = GetBitBool(value,4)
    ppu.mode0Int = GetBitBool(value,3)
    // Bits 2, 1, 0 are read-only
  } else if address == 0xFF44 {
    ppu.LY.write(value)
  } else if address == 0xFF45 {
    ppu.LYC.write(value)
  }
  // TODO
}
