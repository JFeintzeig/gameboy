package cpu 

import (
  "encoding/json"
	"flag"
	"fmt"
  "io"
	//"jfeintzeig/gameboy/internal/cpu"
	"os"
  "testing"
)

var (
  file *string
)

func init() {
  file = flag.String("file","data/Tetris.gb","path to file to load")
}

type TestJson struct {
  Name string `json:"name"`
  Initial State `json:"initial"`
  Final State `json:"final"`
  Cycles [][]interface{} `json:"cycles"`
}

func (t *TestJson) nCycles() uint64 {
  return uint64(len(t.Cycles))
}

type State struct {
  PC uint16 `json:"pc"`
  SP uint16 `json:"SP"`
  A uint8 `json:"a"`
  B uint8 `json:"b"`
  C uint8 `json:"c"`
  D uint8 `json:"d"`
  E uint8 `json:"e"`
  F uint8 `json:"f"`
  H uint8 `json:"h"`
  L uint8 `json:"l"`
  IME uint8 `json:"ime"`
  IE uint8 `json:"ie"`
  Ram []RAM `json:"ram"`
}

type RAM []uint16

func SetInitialState(cpu *Cpu, s State) {
  cpu.PC.write(s.PC)
  cpu.SP.write(s.SP)
  cpu.A.write(s.A)
  cpu.B.write(s.B)
  cpu.C.write(s.C)
  cpu.D.write(s.D)
  cpu.E.write(s.E)
  cpu.F.write(s.F)
  cpu.H.write(s.H)
  cpu.L.write(s.L)
  if s.IME == 0 {
    cpu.IME = false
  } else {
    cpu.IME = true
  }
  cpu.Bus.WriteToBus(IE, s.IE)
  for i := 0; i < len(s.Ram); i += 1 {
    cpu.Bus.WriteToBus(s.Ram[i][0], uint8(s.Ram[i][1]))
  }
}

func CheckState(cpu *Cpu, s State) bool {
  if cpu.PC.read() != s.PC {
    fmt.Printf("PC %d %d ", cpu.PC.read(), s.PC)
    return false
  }
  if cpu.SP.read() != s.SP {
    fmt.Printf("SP %d %d ", cpu.SP.read(), s.SP)
    return false
  }
  if cpu.A.read() != s.A {
    fmt.Printf("A %d %d ", cpu.A.read(), s.A)
    return false
  }
  if cpu.B.read() != s.B {
    fmt.Printf("B %d %d ", cpu.B.read(), s.B)
    return false
  }
  if cpu.C.read() != s.C {
    fmt.Printf("C %d %d ", cpu.C.read(), s.C)
    return false
  }
  if cpu.D.read() != s.D {
    fmt.Printf("D %d %d ", cpu.D.read(), s.D)
    return false
  }
  if cpu.E.read() != s.E {
    fmt.Printf("E %d %d ", cpu.E.read(), s.E)
    return false
  }
  if cpu.F.read() != s.F {
    fmt.Printf("F %d %d ", cpu.F.read(), s.F)
    return false
  }
  if cpu.H.read() != s.H {
    fmt.Printf("H %d %d ", cpu.H.read(), s.H)
    return false
  }
  if cpu.L.read() != s.L {
    fmt.Printf("L %d %d ", cpu.L.read(), s.L)
    return false
  }
  //if cpu.IME && s.IME != 1 {
  //  fmt.Printf("IME %t %d ", cpu.IME, s.IME)
  //  return false
  //}
  //if !cpu.IME && s.IME != 0 {
  //  fmt.Printf("IME %t %d ", cpu.IME, s.IME)
  //  return false
  //}
  //if cpu.Bus.ReadFromBus(IE) != s.IE {
  //  fmt.Printf("IE %d %d ", cpu.Bus.ReadFromBus(IE), s.IE)
  //  return false
  //}
  for i := 0; i < len(s.Ram); i += 1 {
    if cpu.Bus.ReadFromBus(s.Ram[i][0]) != uint8(s.Ram[i][1]) {
      fmt.Printf("RAM %d:%d vs. %d ", s.Ram[i][0], cpu.Bus.ReadFromBus(s.Ram[i][0]), s.Ram[i][1])
      return false
    }
  }
  return true
}

func TestCpu(t *testing.T) {
  flag.Parse()

  jsonFile, err := os.Open(*file)
  if err != nil {
    fmt.Println(err)
  }

  defer jsonFile.Close()

  byteValue, _ := io.ReadAll(jsonFile)

  var tests []TestJson
  json.Unmarshal(byteValue, &tests)

  dummyROM := "/Users/jfeintzeig/projects/2023/gameboy/data/nullbytes_32kb.gb"

  for _, test := range tests {
    cpu := NewGameBoy(&dummyROM, false, true)
    SetInitialState(cpu, test.Initial)
    cpu.Execute(false, test.nCycles())
    if !CheckState(cpu, test.Final) {
      fmt.Printf("%s: %d cycles: ", test.Name, test.nCycles())
      fmt.Printf("FAIL\n")
      t.Errorf("%s", test.Name)
    }
  }
}
