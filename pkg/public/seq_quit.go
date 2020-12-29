package public

import (
    "sort"
    log "github.com/sirupsen/logrus"
)

type FinSig struct {}

type QuitCtx struct {
    idx             int
    DoneChen        chan interface{}
    FinSigChan      chan FinSig
}

func (q *QuitCtx) Done() chan interface{} {
    return q.DoneChen
}

func (q *QuitCtx) Finish() {
    q.FinSigChan <- FinSig{}
}

func (q *QuitCtx) Cancel() {
    select {
    case q.DoneChen <- true:
    default:
    }
}

func NewQuitCtx(priority int) *QuitCtx {
    return &QuitCtx {
        idx:        priority,
        DoneChen:   make(chan interface{}, 1),
        FinSigChan: make(chan FinSig),
    }
}

type SeqQuit struct {
    ctxMap          map[int]*QuitCtx
    keyList         []int
}

func NewSeqQuit() *SeqQuit {
    return &SeqQuit {
        ctxMap:     make(map[int]*QuitCtx),
        keyList:    []int {},
    }
}

func (s *SeqQuit) AddSeq(qc *QuitCtx) {
    if _, ok := s.ctxMap[qc.idx]; !ok {
        s.ctxMap[qc.idx] = qc
        s.keyList = append(s.keyList, qc.idx)
    }
}

func (s *SeqQuit) Quit() {
    sort.Ints(s.keyList)
    log.Info(s.keyList)
    for _, p := range s.keyList {
        qc := s.ctxMap[p]
        qc.Cancel()

        <- qc.FinSigChan
    }
}

