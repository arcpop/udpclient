package main

import (
    "fmt"
    "net"
    "time"
    "sync/atomic"
    "sync"
    "encoding/binary"
	"runtime"
)
var timeout time.Duration
var numSeconds int 
var numRoutines int

var oks uint32
var timeouts uint32
var errors uint32

var packets map[uint64] time.Time
var mux sync.RWMutex

func timerTick(ticker *time.Ticker, stop chan int) {
    i := 0
    for _ = range ticker.C {
        i++;
        onTick(i)
        if i == numSeconds {
            ticker.Stop()
            numRoutines = runtime.NumGoroutine() + 3
            for j := 0; j < numRoutines; j++ {
                stop <- 1
            }
        }
    }
}


func sendClient(conn *net.UDPConn, packetCounter uint64, stop chan int) {
    
    for ;; packetCounter++ {
        select {
            case <- stop:
                return
            default:
        }
        var buffer [1200]byte
        binary.BigEndian.PutUint64(buffer[0:8], packetCounter)
        t := time.Now().Add(timeout)
        mux.Lock()
        packets[packetCounter] = t
        mux.Unlock()
        _, e := conn.Write(buffer[:])
        if e != nil {
            panic(e.Error())
        }
    }
    
}




func recvClient(conn *net.UDPConn, stop chan int) {
    
    for {
    select {
            case <- stop:
                return
            default:
        }
        var buffer [1200]byte
        n, e := conn.Read(buffer[:])
        if e != nil || n < 8 {
            atomic.AddUint32(&errors, 1)
            continue
        }
        v := binary.BigEndian.Uint64(buffer[0:8])
        mux.RLock()
        _, ok := packets[v]
        mux.RUnlock()
        if !ok {
            continue
        }
        atomic.AddUint32(&oks, 1)
    }
}

func onTick(i int) {
    mux.Lock()
    n := time.Now()
    for p, t := range packets {
        if n.After(t) {
            delete(packets, p)
            atomic.AddUint32(&timeouts, 1)
        }
    }
    mux.Unlock()
    fmt.Println("[", i, "] OKs: ", atomic.SwapUint32(&oks, 0), "; Errors: ", atomic.SwapUint32(&errors, 0), "; Timeouts: ", atomic.SwapUint32(&timeouts, 0))
}

func main()  {
    stop := make(chan int)
    ticker := time.NewTicker(time.Second * 1)
    addr, e := net.ResolveUDPAddr("udp4", "localhost:6666")
    if e != nil {
        panic(e.Error())
    }
    conn, e := net.DialUDP("udp4", nil, addr)
    if e != nil {
        panic(e.Error())
    }
    
    timeout = time.Second * 3
    numRoutines = 10
    numSeconds = 30
    oks = 0
    timeouts = 0
    errors = 0
    packets = make(map[uint64] time.Time)
    
    go timerTick(ticker, stop)
    go sendClient(conn, 0, stop)
    go recvClient(conn, stop)
    go recvClient(conn, stop)
    
    <- stop
}
