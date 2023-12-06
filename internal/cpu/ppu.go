package cpu

import (
  "fmt"
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

func NewPixel(lowData uint8, highData uint8) *Pixel {
  return &Pixel{color: highData << 1 | lowData}
}

func NewPpu(busPointer *Bus) *Ppu {
  ppu := Ppu{}

  ppu.currentMode = M2
  ppu.LY.write(0)
  ppu.currentFetcherState = 0
  ppu.statInterruptLine = false

  ppu.bus = busPointer
  ppu.screen = [160*144]uint8{}
  ppu.writtenPixels = make(map[uint8]uint8)

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

  // LCD rendering
  screen [160*144]uint8
  renderX uint8
  writtenPixels map[uint8]uint8

  nDots uint16

  // fetcher state
  currentFetcherState FetcherState
  fetcherX uint8
  CurrentTileIndex uint8
  CurrentTileDataLow uint8
  CurrentTileDataHigh uint8

  applyFetcherState [4]func() bool

  bgFifo Fifo[*Pixel]
  objFifo Fifo[*Pixel]

  // LCDC
  lcdEnable bool
  windowTileMap bool
  windowEnable bool
  bgWinDataAddress bool
  bgTileMap bool
  objSize bool
  objEnable bool
  bgWinDisplay bool

  LY Register8
  LYC Register8
  SCX Register8
  SCY Register8
  WY Register8
  WX Register8

  // STAT
  lycInt bool
  mode2Int bool
  mode1Int bool
  mode0Int bool
  LYCeqLY bool
  currentMode Mode
  statInterruptLine bool
}

func (ppu *Ppu) XInsideWindow() bool {
  // TODO
  return false
}

func (ppu *Ppu) GetTile() bool {
  var bgTileMapAddress uint16 = 0x9800
  if (ppu.bgTileMap && !ppu.XInsideWindow()) || (ppu.windowTileMap && ppu.XInsideWindow()) {
    bgTileMapAddress = 0x9C00
  }

  var tileNum uint16

  // TODO: is this if statement right?
  if ppu.XInsideWindow() {
    // TODO: window
    tileNum = 0
  } else {
    // tile map is 32 x 32, so one X is 8 pixels or 1 fetcherX and
    // one Y is 32 LY's divided by 8 pixels
    // something not right here...how does fetcherX increment?
    // TODO: how to increment fetcherX?
    tileNum = uint16((ppu.SCX.read() / 8 + ppu.fetcherX) & 0x1F)
    tileNum += 32 * uint16((ppu.LY.read() + ppu.SCY.read()) & 0xFF) / 8
  }

  // keep it in the 32 x 32 range, so < 1024
  tileNum &= 0x3FFF

  ppu.CurrentTileIndex = ppu.bus.ReadFromBus(bgTileMapAddress + tileNum)
  //fmt.Printf("%X %X\n", tileNum, ppu.CurrentTileIndex)
  return true
}

func (ppu *Ppu) GetTileData() uint8 {
  var baseAddress uint16 = 0x8800
  if ppu.bgWinDataAddress {
    baseAddress = 0x8000
  }

  // TODO: finish this line, this is probably not right
  offset := uint16(ppu.CurrentTileIndex)*16 + 2 * (uint16(ppu.LY.read() + ppu.SCY.read()) % 8)
  tileData := ppu.bus.ReadFromBus(baseAddress + offset)
  return tileData
}

func (ppu *Ppu) GetTileDataLow() bool {
  ppu.CurrentTileDataLow = ppu.GetTileData()
  return true
}

func (ppu *Ppu) GetTileDataHigh() bool {
  ppu.CurrentTileDataHigh = ppu.GetTileData()
  return true
}

// TODO: control-flow, how to make this
// "attempted every dot until it succeeds"?
func (ppu *Ppu) Push() bool {
  if ppu.bgFifo.Length() > 0 {
    return false
  }

  // combine CurrenTileDataLow and High into pixels
  // push to fifo
  for i := 0; i < 8; i ++ {
    low := (ppu.CurrentTileDataLow >> (7-i)) & 0x01
    high := (ppu.CurrentTileDataHigh >> (7-i)) & 0x01

    ppu.bgFifo.Push(NewPixel(low, high))
  }

  // TODO: is this right?
  ppu.fetcherX += 1
  ppu.fetcherX = ppu.fetcherX % 32
  return true
}

func (ppu *Ppu) renderPixelToScreen() {
  if ppu.bgFifo.Length() == 0 {
    return
  }
  if ppu.renderX > 159 {
    ppu.renderX = 0
    return
  }

  pixel := ppu.bgFifo.Pop()

  // TODO: get color from pallete
  var color uint8
  if !ppu.bgWinDisplay {
    // draw color 0 pixel
    color = 0x00
  } else {
    color = pixel.color
  }

  coord := ppu.LY.read()*160 + ppu.renderX

  ppu.screen[ppu.LY.read()*160 + ppu.renderX] = color
  if color != 0 {
    //fmt.Printf("good: %d\n", ppu.screen[coord])
    ppu.writtenPixels[coord] = color
  }

  oldColor, ok := ppu.writtenPixels[coord]
  if ok {
    if oldColor != color {
      //fmt.Printf("overwriting pixel %d from %d to %d\n",coord, oldColor, color)
    }
  }

  ppu.renderX += 1
}

func (ppu *Ppu) doFetchRoutine() {
  success := ppu.applyFetcherState[ppu.currentFetcherState]()

  if ppu.currentFetcherState == Push && !success {
    return
  }

  ppu.currentFetcherState = ppu.currentFetcherState.next()
}

func (ppu *Ppu) maybeRequestInterrupt() {
  newStatInterruptLine := false

  if ppu.lycInt && ppu.LYCeqLY {
    newStatInterruptLine = true
  } else if ppu.currentMode == M0 && ppu.mode0Int {
    newStatInterruptLine = true
  } else if ppu.currentMode == M1 && ppu.mode1Int {
    newStatInterruptLine = true
  } else if ppu.currentMode == M2 && ppu.mode2Int {
    newStatInterruptLine = true
  }

  if newStatInterruptLine && !ppu.statInterruptLine {
    ppu.statInterruptLine = true
    IF := ppu.bus.ReadFromBus(0xFF0F)
    ppu.bus.WriteToBus(0xFF0F, SetBitBool(IF, 1, true))
  }

  ppu.statInterruptLine = newStatInterruptLine
}

func (ppu *Ppu) doCycle() {
  //fmt.Printf("M:%d FS: %d LY:%d FX: %d TI: %d\n", ppu.currentMode, ppu.currentFetcherState, ppu.LY.read(), ppu.fetcherX, ppu.CurrentTileIndex)
  ppu.nDots += 4

  if ppu.LYC == ppu.LY {
    ppu.LYCeqLY = true
  } else {
    ppu.LYCeqLY = false
  }

  ppu.maybeRequestInterrupt()

  if ppu.currentMode == M2 {
    // implement M2

    if ppu.nDots == 80 {
      ppu.nDots = 0
      ppu.currentMode = M3
    }
  } else if ppu.currentMode == M3 {
    // 4 dots worth
    ppu.doFetchRoutine()
    ppu.doFetchRoutine()

    for i := 0; i < 4; i++ {
      ppu.renderPixelToScreen()
    }

    if ppu.nDots == 172 { // TODO: penalties
      ppu.currentMode = M0
      ppu.currentFetcherState = 0
      // don't reset nDots here, keep counting to end of line
    }
  } else if ppu.currentMode == M0 {
    // HBlank - do stuff

    // at end of routine, if 
    if ppu.nDots == 376 {
      ppu.LY.inc()
      ppu.nDots = 0
      // TODO: maybe need this?
      ppu.fetcherX = 0

      if ppu.LY.read() == 144 {
        ppu.currentMode = M1

        // VBlank interrupt
        IF := ppu.bus.ReadFromBus(0xFF0F)
        ppu.bus.WriteToBus(0xFF0F, SetBitBool(IF, 0, true))
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

      ppu.LogScreen()

    } else if ppu.nDots % 456 == 0 {
      // end of scanline
      ppu.LY.inc()
    }
  }
}

func (ppu *Ppu) read(address uint16) uint8 {
  if address == LCDC {
    var result uint8
    result = SetBitBool(result, 7, ppu.lcdEnable)
    result = SetBitBool(result, 6, ppu.windowTileMap)
    result = SetBitBool(result, 5, ppu.windowEnable)
    result = SetBitBool(result, 4, ppu.bgWinDataAddress)
    result = SetBitBool(result, 3, ppu.bgTileMap)
    result = SetBitBool(result, 2, ppu.objSize)
    result = SetBitBool(result, 1, ppu.objEnable)
    result = SetBitBool(result, 0, ppu.bgWinDisplay)
    return result
  } else if address == STAT {
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
  } else if address == SCX {
    return ppu.SCX.read()
  } else if address == SCY {
    return ppu.SCY.read()
  } else if address == WX {
    return ppu.WX.read()
  } else if address == WY {
    return ppu.WY.read()
  }
  // TODO
  return 0xFF
}

func (ppu *Ppu) write(address uint16, value uint8) {
  if address == LCDC {
    ppu.lcdEnable = GetBitBool(value, 7)
    ppu.windowTileMap = GetBitBool(value, 6)
    ppu.windowEnable = GetBitBool(value, 5)
    ppu.bgWinDataAddress = GetBitBool(value, 4)
    ppu.bgTileMap = GetBitBool(value, 3)
    ppu.objSize = GetBitBool(value, 2)
    ppu.objEnable = GetBitBool(value, 1)
    ppu.bgWinDisplay = GetBitBool(value, 0)
  } else if address == STAT {
    ppu.lycInt = GetBitBool(value,6)
    ppu.mode2Int = GetBitBool(value,5)
    ppu.mode1Int = GetBitBool(value,4)
    ppu.mode0Int = GetBitBool(value,3)
    // Bits 2, 1, 0 are read-only
  } else if address == LY {
    ppu.LY.write(value)
  } else if address == LYC {
    ppu.LYC.write(value)
  } else if address == SCX {
    ppu.SCX.write(value)
  } else if address == SCY {
    ppu.SCY.write(value)
  } else if address == WX {
    ppu.WX.write(value)
  } else if address == WY {
    ppu.WY.write(value)
  }
  // TODO
}

func (ppu *Ppu) PrintBgData() {
}

func (ppu *Ppu) LogScreen() {
  fmt.Printf("log screen\n")
  for i, val := range ppu.screen {
    if val != 0 {
      fmt.Printf("%d: %d\n",i, val)
    }
  }
  //fmt.Printf("%v", ppu.screen)
  //for i := 0; i < 144; i++ {
  //  for j := 0; j < 160; j++ {
  //    pixel := ppu.screen[i*159 + j]
  //    if pixel != 0 {
  //      fmt.Printf("%d",pixel)
  //    }
  //  }
  //  fmt.Printf("\n")
  //}
}
