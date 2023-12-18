package cpu

import (
  "fmt"
  //"runtime"
)

type Mode int

const N_MODES = 4

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
  renderX uint16
  scrollDiscardedX uint8
  renderingWindow bool

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

func (ppu *Ppu) UsingWindow() bool {
  // TODO: should we check x-coord here or later?
  if ppu.windowEnable && ppu.fetcherX*8 >= (max(ppu.WX.read(),7) - 7) && ppu.LY.read() >= ppu.WY.read() {
    return true
  } else {
    return false
  }
}

func (ppu *Ppu) GetTile() bool {
  var bgTileMapAddress uint16 = 0x9800
  if (ppu.bgTileMap && !ppu.UsingWindow()) || (ppu.UsingWindow() && ppu.windowTileMap) {
    bgTileMapAddress = 0x9C00
  }

  var tileMapAddressOffset uint16

  if ppu.UsingWindow() {
    tileMapAddressOffset = uint16(ppu.fetcherX - ppu.WX.read()/8)
    tileMapAddressOffset += 32 * (uint16(ppu.LY.read() - ppu.WY.read()) / 8)
  } else {
    // tile map is 32 x 32, so one X is 8 pixels or 1 fetcherX and
    // one Y is 32 LY's divided by 8 pixels
    tileMapAddressOffset = uint16((ppu.SCX.read() / 8 + ppu.fetcherX) & 0x1F)
    tileMapAddressOffset += 32 * (uint16((ppu.LY.read() + ppu.SCY.read()) & 0xFF) / 8)
  }

  tileMapAddressOffset &= 0x3FF

  ppu.CurrentTileIndex = ppu.bus.ReadFromBus(bgTileMapAddress + tileMapAddressOffset)
  return true
}

func (ppu *Ppu) GetTileData(offset uint16) uint8 {
  var baseAddress uint16
  var tileIndexOffset uint16
  var tileData uint8
  var finalAddress uint16

  if ppu.bgWinDataAddress {
    baseAddress = 0x8000
    yOffset := uint16(ppu.LY.read() + ppu.SCY.read())
    if ppu.UsingWindow() {
      yOffset = uint16(ppu.LY.read() - ppu.WY.read())
    }
    tileIndexOffset = 16 * uint16(ppu.CurrentTileIndex) + 2 * (yOffset % 8)
    finalAddress = baseAddress + tileIndexOffset + offset
  } else {
    baseAddress = 0x9000
    yOffset := uint16(ppu.LY.read() + ppu.SCY.read())
    if ppu.UsingWindow() {
      yOffset = uint16(ppu.LY.read() - ppu.WY.read())
    }
    if ppu.CurrentTileIndex > 0x7F {
      tileIndexOffset = 16 * (256-uint16(ppu.CurrentTileIndex)) + 2 * (yOffset % 8)
      finalAddress = baseAddress - tileIndexOffset + offset
    } else {
      tileIndexOffset = 16 * uint16(ppu.CurrentTileIndex) + 2 * (yOffset % 8)
      finalAddress = baseAddress + tileIndexOffset + offset
    }
  }
  tileData = ppu.bus.ReadFromBus(finalAddress)

  return tileData
}

func (ppu *Ppu) GetTileDataLow() bool {
  ppu.CurrentTileDataLow = ppu.GetTileData(0)
  return true
}

func (ppu *Ppu) GetTileDataHigh() bool {
  ppu.CurrentTileDataHigh = ppu.GetTileData(1)

  return true
}

func (ppu *Ppu) Push() bool {
  if ppu.bgFifo.Length() > 0 {
    return false
  }

  for i := 0; i < 8; i ++ {
    low := (ppu.CurrentTileDataLow >> (7-i)) & 0x01
    high := (ppu.CurrentTileDataHigh >> (7-i)) & 0x01

    ppu.bgFifo.Push(NewPixel(low, high))
  }

  ppu.fetcherX += 1

  return true
}

func (ppu *Ppu) clearFifo() {
  for ppu.bgFifo.Length() > 0 {
    ppu.bgFifo.Pop()
  }
}

func (ppu *Ppu) renderPixelToScreen() {
  if ppu.bgFifo.Length() == 0 {
    return
  }
  // SCX scrolling penalty: discard first pixels in given tile
  if ppu.scrollDiscardedX < (ppu.SCX.read() % 8) {
    ppu.bgFifo.Pop()
    ppu.scrollDiscardedX += 1
    return
  }
  // window penalty
  if ppu.UsingWindow() && !ppu.renderingWindow {
    ppu.clearFifo()
    ppu.currentFetcherState = 0
    ppu.renderingWindow = true
    return
  }
  if ppu.renderX > 159 {
    return
  }

  pixel := ppu.bgFifo.Pop()

  // TODO: get color from pallete
  var color uint8
  if !ppu.bgWinDisplay {
    color = 0x00
  } else {
    color = pixel.color
  }

  coord := uint16(ppu.LY.read())*160 + ppu.renderX
  ppu.screen[coord] = color

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
    // first 6 dots do nothing so by the
    // time nDots = 8 we should have done 1 routine
    if ppu.nDots == 4 {
      return
    }
    if ppu.nDots == 8 {
      ppu.doFetchRoutine()
      return
    }

    // 4 dots worth
    ppu.doFetchRoutine()
    ppu.renderPixelToScreen()
    ppu.renderPixelToScreen()

    ppu.doFetchRoutine()
    ppu.renderPixelToScreen()
    ppu.renderPixelToScreen()

    if ppu.renderX == 160 { // TODO: penalties
      ppu.currentMode = M0
      ppu.currentFetcherState = 0
      ppu.fetcherX = 0
      ppu.renderX = 0
      ppu.scrollDiscardedX = 0
      ppu.renderingWindow = false
      ppu.clearFifo()
      // don't reset nDots here, keep counting to end of line
    }
  } else if ppu.currentMode == M0 {
    // HBlank - do stuff

    // at end of routine, if 
    if ppu.nDots == 376 {
      ppu.LY.inc()
      ppu.nDots = 0

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

      //ppu.RenderEasy()

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
    // LY is read-only
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

// TODO: so with this logging for
// hello world, i know the true values of:
// tileMap addr -> tileMap index -> tileData addr ->
// (x,y) pos on screen -> index in screen slice.
// i need to log the same info in this routine vs.
// in PPU and compare for each tile
// another debug idea: add filters so PPU renders
// just one tile, see where it ends up and how it looks
func (ppu *Ppu) RenderEasy() {
  address := uint16(0x9800)
  pos := 0
  // its a 32 x 32 tilemap
  // but we dont want to draw past edge of screen
  // which is 20 tiles horizontal and 18 vertical
  for pos < 32*18 {
    tileIndex := ppu.bus.ReadFromBus(address)
    fmt.Printf("*********************************\n")
    fmt.Printf("Tile in pos %d, index address 0x%04x\n", pos, address)
    ppu.DrawTileData(uint16(tileIndex), pos)
    address += 1
    pos += 1
  }
}

func (ppu *Ppu) DrawTileData(tileIndex uint16, pos int) {
  // dont draw past 20 tiles horizontally
  if pos % 32 > 19 {
    return
  }
  address := uint16(0x9000) + tileIndex*16
  stopAddress := address + 16
  // coord is in address space, so 2 X # of pixels
  coord := pos*16

  fmt.Printf("index %d\n", tileIndex)

  for address < stopAddress {
    tileNum := coord / 16  // 16 bytes per tile
    xBase := (tileNum%32) * 8 // 32 sprites per line in tileMap
    yBase := (tileNum/32) * 160 * 8 // 160 pixels per line
    // should be between 0 and 1280 always
    // should be multiple of 160 b/c its adding rows
    offset := (160 * (coord - yBase%1280)/2) % 1280 // how many rows into sprite

    low := ppu.bus.ReadFromBus(address)
    address += 1
    high := ppu.bus.ReadFromBus(address)
    address += 1
    fmt.Printf("Tiledata @0x%04X: %02X %02X ", address-2, low, high)

    fmt.Printf("TileNum:%d xBase:%d yBase:%d offset:%d\n", tileNum, xBase, yBase, offset)

    for i:= 0; i < 8; i++ {
      lb := (low >> (7-i)) & 0x01
      hb := (high >> (7-i)) & 0x01
      pixel := (hb << 1 | lb)
      index := yBase + xBase + offset + i
      ppu.screen[index] = pixel
    }

    coord += 2
  }
}
