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
  priority uint8
}

type palette map[uint8]uint8

func (p palette) read() uint8 {
  var value uint8 = 0x0
  if c0, ok := p[0]; ok {
    value = value | c0
  }
  if c1, ok := p[1]; ok {
    value = value | (c1 << 2)
  }
  if c2, ok := p[2]; ok {
    value = value | (c2 << 4)
  }
  if c3, ok := p[3]; ok {
    value = value | (c3 << 6)
  }
  return value
}

func (p palette) write(value uint8) {
  p[0] = value & 0x03
  p[1] = (value & 0x0C) >> 2
  p[2] = (value & 0x30) >> 4
  p[3] = (value & 0xC0) >> 6
}

func NewPpu(busPointer *Bus) *Ppu {
  ppu := Ppu{}

  ppu.vram = [8*1024]Register8{}
  ppu.oam = [160]Register8{}

  ppu.currentMode = M2
  ppu.LY.write(0)
  ppu.currentFetcherState = 0
  ppu.statInterruptLine = false

  ppu.bus = busPointer
  ppu.screen = [160*144]uint8{}

  ppu.bgp = make(palette)
  ppu.obp0 = make(palette)
  ppu.obp1 = make(palette)

  ppu.applyFetcherState = [N_FETCHER_STATES]func()bool{
    ppu.GetTile,
    ppu.GetTileDataLow,
    ppu.GetTileDataHigh,
    ppu.Push,
  }

  return &ppu
}

type Sprite struct {
  yPos uint8
  xPos uint8
  tileIndex uint8
  flags uint8
}

type Ppu struct {
  bus Mediator

  vram [8*1024]Register8
  oam [160]Register8

  // LCD rendering
  screen [160*144]uint8
  renderX uint16
  scrollDiscardedX uint8
  renderingWindow bool
  renderedWindowThisLY bool

  nDots uint16

  // fetcher state
  currentFetcherState FetcherState
  fetcherX uint8
  windowLineCounter uint8
  CurrentTileIndex uint8
  CurrentTileDataLow uint8
  CurrentTileDataHigh uint8

  fetchingSprite bool
  SpriteToRender Sprite

  applyFetcherState [4]func() bool

  bgFifo Fifo[*Pixel]
  spriteFifo Fifo[*Pixel]

  // OAM scan
  SpriteBuffer []Sprite
  OAMOffset uint8

  // LCDC
  lcdEnable bool
  windowTileMap bool
  windowEnable bool
  bgWinDataAddress bool
  bgTileMap bool
  objSize bool
  spriteEnable bool
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

  // palette's
  bgp palette
  obp0 palette
  obp1 palette
}

func (ppu *Ppu) InsideWindow() bool {
  // TODO: should we check x-coord here or later?
  if ppu.windowEnable && uint8(ppu.renderX) >= (max(ppu.WX.read(),7) - 7) && ppu.LY.read() >= ppu.WY.read() {
    return true
  } else {
    return false
  }
}

func (ppu *Ppu) GetTile() bool {
  if ppu.fetchingSprite {
    ppu.CurrentTileIndex = ppu.SpriteToRender.tileIndex
    if ppu.objSize {
      ppu.CurrentTileIndex = SetBit(ppu.CurrentTileIndex, 0, 0)
    }
    return true
  } else {
    var bgTileMapAddress uint16 = 0x9800
    if (ppu.bgTileMap && !ppu.renderingWindow) || (ppu.renderingWindow && ppu.windowTileMap) {
      bgTileMapAddress = 0x9C00
    }

    var tileMapAddressOffset uint16

    if ppu.renderingWindow {
      // offset is based on "window X", which is diff. between fetcherX and WX
      tileMapAddressOffset = uint16(ppu.fetcherX - (ppu.WX.read()-7)/8)
      tileMapAddressOffset += 32 * (uint16(ppu.windowLineCounter) / 8)
    } else {
      // tile map is 32 x 32, so one X is 8 pixels or 1 fetcherX and
      // one Y is 32 LY's divided by 8 pixels
      tileMapAddressOffset = uint16((ppu.SCX.read() / 8 + ppu.fetcherX) & 0x1F)
      tileMapAddressOffset += 32 * (uint16((ppu.LY.read() + ppu.SCY.read()) & 0xFF) / 8)
    }
    //fmt.Printf("LY %d fetcherX %d tileMapAddr %04X ", ppu.LY.read(), ppu.fetcherX, bgTileMapAddress + tileMapAddressOffset)

    tileMapAddressOffset &= 0x3FF

    ppu.CurrentTileIndex = ppu.read(bgTileMapAddress + tileMapAddressOffset)
    return true
  }
}

func (ppu *Ppu) GetTileData(offset uint16) uint8 {
  var baseAddress uint16
  var tileIndexOffset uint16
  var tileData uint8
  var finalAddress uint16

  if ppu.fetchingSprite {
    baseAddress = 0x8000

    var SpriteHeight uint16
    if ppu.objSize {
      SpriteHeight = 16
    } else {
      SpriteHeight = 8
    }
    yOffset := uint16(ppu.LY.read() - ppu.SpriteToRender.yPos - 16) % SpriteHeight
    // Y flip
    if GetBitBool(ppu.SpriteToRender.flags, 6) {
      yOffset = SpriteHeight - yOffset - 1
    }

    tileIndexOffset = 16 * uint16(ppu.CurrentTileIndex) + 2 * yOffset
    finalAddress = baseAddress + tileIndexOffset + offset
  } else if ppu.bgWinDataAddress {
    baseAddress = 0x8000
    yOffset := uint16(ppu.LY.read() + ppu.SCY.read())
    if ppu.renderingWindow {
      yOffset = uint16(ppu.windowLineCounter)
    }
    tileIndexOffset = 16 * uint16(ppu.CurrentTileIndex) + 2 * (yOffset % 8)
    finalAddress = baseAddress + tileIndexOffset + offset
  } else {
    baseAddress = 0x9000
    yOffset := uint16(ppu.LY.read() + ppu.SCY.read())
    if ppu.renderingWindow {
      yOffset = uint16(ppu.windowLineCounter)
    }
    if ppu.CurrentTileIndex > 0x7F {
      tileIndexOffset = 16 * (256-uint16(ppu.CurrentTileIndex)) - 2 * (yOffset % 8)
      finalAddress = baseAddress - tileIndexOffset + offset
    } else {
      tileIndexOffset = 16 * uint16(ppu.CurrentTileIndex) + 2 * (yOffset % 8)
      finalAddress = baseAddress + tileIndexOffset + offset
    }
  }
  tileData = ppu.read(finalAddress)

  if !ppu.fetchingSprite {
    //fmt.Printf("tileIndex %d tileAddr %04X tileData %02X ", ppu.CurrentTileIndex, finalAddress, tileData)
  }

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
  if ppu.fetchingSprite {
    // TODO: mixing! edge cases!
    for i := 0; i < 8; i ++ {
      // if xPos is 3, only last 3 pixels are rendered
      if int(ppu.SpriteToRender.xPos) + i - 8 < 0 {
        continue
      }
      // don't load pixels if slot already occupied
      // how to do this? b/c spriteFifo.Length() will change as we push!
      if i < ppu.spriteFifo.Length() {
        continue
      }

      // X flip
      var offset int
      if GetBitBool(ppu.SpriteToRender.flags, 5) {
        offset = i
      } else {
        offset = 7-i
      }

      low := (ppu.CurrentTileDataLow >> offset) & 0x01
      high := (ppu.CurrentTileDataHigh >> offset) & 0x01
      palette := GetBit(ppu.SpriteToRender.flags, 4)
      priority := GetBit(ppu.SpriteToRender.flags, 7)
      ppu.spriteFifo.Push(&Pixel{color: high << 1 | low, palette: palette, priority: priority})
    }

    // after this we're done fetching this sprite
    ppu.fetchingSprite = false
    // we don't increment fetcherX b/c thats for background
    return true
  }

  if ppu.bgFifo.Length() > 0 {
    return false
  }

  ///fmt.Printf("pixels: ")
  for i := 0; i < 8; i ++ {
    low := (ppu.CurrentTileDataLow >> (7-i)) & 0x01
    high := (ppu.CurrentTileDataHigh >> (7-i)) & 0x01

    ppu.bgFifo.Push(&Pixel{color: high << 1 | low})
    //fmt.Printf("%d ", high << 1 | low)
  }

  //fmt.Printf("\n")
  ppu.fetcherX += 1

  return true
}

func (ppu *Ppu) clearFifo(sprite bool) {
  for ppu.bgFifo.Length() > 0 {
    ppu.bgFifo.Pop()
  }

  if sprite {
    for ppu.spriteFifo.Length() > 0 {
      ppu.spriteFifo.Pop()
    }
  }
}

func (ppu *Ppu) isTimeToRenderSprite() (bool, int) {
  for idx, sp := range ppu.SpriteBuffer {
    if sp.xPos <= uint8(ppu.renderX) + 8 {
      return true, idx
    }
  }
  return false, 0
}

func (ppu *Ppu) renderPixelToScreen() {
  if ppu.bgFifo.Length() == 0 {
    return
  }
  // SCX scrolling penalty: discard first pixels in given tile
  if ppu.scrollDiscardedX < (ppu.SCX.read() % 8) {
    //fmt.Printf("discard scroll pixel\n")
    ppu.bgFifo.Pop()
    ppu.scrollDiscardedX += 1
    return
  }

  // when first start rendering window, restart Fifo + fetch
  if ppu.InsideWindow() && !ppu.renderingWindow {
    ppu.clearFifo(false)
    // reset fetcherX: since fetcher compares fetcherX to WX to decide
    // whether to us window or not, we reset fetcherX to start of window
    ppu.fetcherX = (ppu.WX.read() - 7)/8
    ppu.currentFetcherState = GetTile
    ppu.renderingWindow = true
    ppu.renderedWindowThisLY = true
    return
  }
  if !ppu.InsideWindow() && ppu.renderingWindow {
    ppu.renderingWindow = false
  }

  // rendering paused until done fetching sprite
  if ppu.fetchingSprite {
    //fmt.Printf("fetching sprite\n")
    return
  }
  // initiate sprite fetch
  if isTime, spriteIdx := ppu.isTimeToRenderSprite(); isTime {
    ppu.fetchingSprite = true
    ppu.SpriteToRender = ppu.SpriteBuffer[spriteIdx]
    ppu.currentFetcherState = GetTile
    // remove `sprite` from `SpriteBuffer`
    ppu.SpriteBuffer = append(ppu.SpriteBuffer[:spriteIdx], ppu.SpriteBuffer[spriteIdx+1:]...)
    return
  }

  if ppu.renderX > 159 {
    //fmt.Printf("renderX too big\n")
    return
  }

  bgPixel := ppu.bgFifo.Pop()
  var color uint8 = 0x00
  if ppu.bgWinDisplay {
    color = ppu.bgp[bgPixel.color]
  } else {
    //fmt.Printf("bg not enabled\n")
  }
  if ppu.spriteFifo.Length() > 0 && ppu.spriteEnable {
    sPixel := ppu.spriteFifo.Pop()
    if sPixel.color != 0x00 && !(sPixel.priority == 0x01 && bgPixel.color != 0x00) {
      if sPixel.palette == 0 {
        color = ppu.obp0[sPixel.color]
      } else {
        color = ppu.obp1[sPixel.color]
      }
    }
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
    IF := ppu.bus.ReadFromBus(0xFF0F)
    ppu.bus.WriteToBus(0xFF0F, SetBitBool(IF, 1, true))
    //if ppu.lycInt && ppu.LYCeqLY {
    //  fmt.Printf("LYC %d LY %d int ", ppu.LYC.read(), ppu.LY.read())
    //}
    //fmt.Printf("written to IF: IE %08b IF %08b\n", ppu.bus.ReadFromBus(0xFFFF), ppu.bus.ReadFromBus(0xFF0F))
  }

  ppu.statInterruptLine = newStatInterruptLine
}

func (ppu *Ppu) scanOAM() {
  if len(ppu.SpriteBuffer) >= 10 {
    return
  }

  yCoord := ppu.read(OAM_START + uint16(ppu.OAMOffset))
  ppu.OAMOffset += 1
  xCoord := ppu.read(OAM_START + uint16(ppu.OAMOffset))
  ppu.OAMOffset += 1
  tileIndex := ppu.read(OAM_START + uint16(ppu.OAMOffset))
  ppu.OAMOffset += 1
  flags := ppu.read(OAM_START + uint16(ppu.OAMOffset))
  ppu.OAMOffset += 1

  LYP16 := ppu.LY.read() + 16
  var SpriteHeight uint8
  if ppu.objSize {
    SpriteHeight = 16
  } else {
    SpriteHeight = 8
  }

  if xCoord >= 0 && LYP16 >= yCoord && LYP16 < yCoord + SpriteHeight {
    sprite := Sprite{yCoord, xCoord, tileIndex, flags}
    ppu.SpriteBuffer = append(ppu.SpriteBuffer, sprite)
  }
  return
}

func (ppu *Ppu) doCycle() {
  if !ppu.lcdEnable {
    return
  }

  ppu.nDots += 4

  if ppu.LYC.read() == ppu.LY.read() {
    ppu.LYCeqLY = true
  } else {
    ppu.LYCeqLY = false
  }

  ppu.maybeRequestInterrupt()

  if ppu.currentMode == M2 {
    // 4 dots worth
    ppu.scanOAM()
    ppu.scanOAM()

    if ppu.nDots == 80 {
      ppu.nDots = 0
      ppu.OAMOffset = 0
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
      if ppu.renderedWindowThisLY {
        ppu.windowLineCounter += 1
      }
      ppu.renderedWindowThisLY = false
      ppu.renderingWindow = false
      ppu.clearFifo(true)
      // don't reset nDots here, keep counting to end of line
    }
  } else if ppu.currentMode == M0 {
    // HBlank - do stuff

    // end of scanline
    if ppu.nDots == 376 {
      ppu.LY.inc()
      ppu.nDots = 0
      ppu.SpriteBuffer = make([]Sprite, 0)

      if ppu.LY.read() == 144 {
        ppu.currentMode = M1
        ppu.windowLineCounter = 0

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

func (ppu *Ppu) clearState() {
    ppu.nDots = 0
    ppu.currentMode = M1
    ppu.LY.write(0)
    ppu.currentFetcherState = 0
    ppu.statInterruptLine = false
    ppu.screen = [160*144]uint8{}
    ppu.OAMOffset = 0
    ppu.fetcherX = 0
    ppu.renderX = 0
    ppu.scrollDiscardedX = 0
    ppu.renderingWindow = false
    ppu.fetchingSprite = false
    ppu.clearFifo(true)
    ppu.SpriteBuffer = make([]Sprite, 0)
}

func (ppu *Ppu) read(address uint16) uint8 {
  switch {
  case address >= 0x8000 && address <= 0x9FFF:
    return ppu.vram[address - 0x8000].read()
  case address >= 0xFE00 && address <= 0xFE9F:
    return ppu.oam[address - 0xFE00].read()
  case address == LCDC:
    var result uint8
    result = SetBitBool(result, 7, ppu.lcdEnable)
    result = SetBitBool(result, 6, ppu.windowTileMap)
    result = SetBitBool(result, 5, ppu.windowEnable)
    result = SetBitBool(result, 4, ppu.bgWinDataAddress)
    result = SetBitBool(result, 3, ppu.bgTileMap)
    result = SetBitBool(result, 2, ppu.objSize)
    result = SetBitBool(result, 1, ppu.spriteEnable)
    result = SetBitBool(result, 0, ppu.bgWinDisplay)
    return result
  case address == STAT:
    var result uint8 = 0
    result = SetBitBool(result, 6, ppu.lycInt)
    result = SetBitBool(result, 5, ppu.mode2Int)
    result = SetBitBool(result, 4, ppu.mode1Int)
    result = SetBitBool(result, 3, ppu.mode0Int)
    result = SetBitBool(result, 2, ppu.LYCeqLY)
    if ppu.lcdEnable {
      result |= uint8(ppu.currentMode)
    }
    return result
  case address == 0xFF44:
    return ppu.LY.read()
  case address == 0xFF45:
    return ppu.LYC.read()
  case address == SCX:
    return ppu.SCX.read()
  case address == SCY:
    return ppu.SCY.read()
  case address == WX:
    return ppu.WX.read()
  case address == WY:
    return ppu.WY.read()
  case address == BGP:
    return ppu.bgp.read()
  case address == OBP0:
    return ppu.obp0.read()
  case address == OBP1:
    return ppu.obp1.read()
  // TODO
  default:
    return 0xFF
  }
}

func (ppu *Ppu) write(address uint16, value uint8) {
  switch {
  case address >= 0x8000 && address <= 0x9FFF:
    ppu.vram[address - 0x8000].write(value)
  case address >= 0xFE00 && address <= 0xFE9F:
    ppu.oam[address - 0xFE00].write(value)
  case address == LCDC:
    ppu.lcdEnable = GetBitBool(value, 7)
    if !ppu.lcdEnable {
      ppu.clearState()
      ppu.windowTileMap = false
      ppu.windowEnable = false
      ppu.bgWinDataAddress = false
      ppu.bgTileMap = false
      ppu.objSize = false
      ppu.spriteEnable = false
      ppu.bgWinDisplay = false
      return
    }
    ppu.windowTileMap = GetBitBool(value, 6)
    ppu.windowEnable = GetBitBool(value, 5)
    ppu.bgWinDataAddress = GetBitBool(value, 4)
    ppu.bgTileMap = GetBitBool(value, 3)
    ppu.objSize = GetBitBool(value, 2)
    ppu.spriteEnable = GetBitBool(value, 1)
    ppu.bgWinDisplay = GetBitBool(value, 0)
  case address == STAT:
    ppu.lycInt = GetBitBool(value,6)
    ppu.mode2Int = GetBitBool(value,5)
    ppu.mode1Int = GetBitBool(value,4)
    ppu.mode0Int = GetBitBool(value,3)
    // Bits 2, 1, 0 are read-only
  case address == LY:
    // LY is read-only
    return
  case address == LYC:
    ppu.LYC.write(value)
  case address == SCX:
    ppu.SCX.write(value)
  case address == SCY:
    ppu.SCY.write(value)
  case address == WX:
    ppu.WX.write(value)
  case address == WY:
    ppu.WY.write(value)
  case address == BGP:
    ppu.bgp.write(value)
  case address == OBP0:
    ppu.obp0.write(value)
  case address == OBP1:
    ppu.obp1.write(value)
  // TODO
  }
}
