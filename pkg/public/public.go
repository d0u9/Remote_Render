package public

import (
    "fmt"
)

type Stage      int

const (
    STG_INIT        Stage = iota
    STG_COPT_TO
    STG_RENDER
    STG_COPY_FROM
    STG_DONE
)

type Task struct {
    AllNr           int
    Idx             int
    DieOn           Stage
    SrcFile         string
    RmtFile         string
    RenderedFile    string
    DstFile         string
    Size            int64
    Err             error
}

func (t *Task) StgString() string {
    switch t.DieOn {
    case STG_INIT:
        return "STG_INIT"
    case STG_COPT_TO:
        return "STG_COPT_TO"
    case STG_RENDER:
        return "STG_RENDER"
    case STG_COPY_FROM:
        return "STG_COPY_FROM"
    case STG_DONE:
        return "STG_DONE"
    }
    return "STG_???"
}

func (t *Task) String() string {
    return fmt.Sprintf("\nStage: %s, SrcFile: %s, RmtFile: %s, RenderedFile: %s, DstFile: %s, Err: %v", t.StgString(), t.SrcFile, t.RmtFile, t.RenderedFile, t.DstFile, t.Err)
}

func ByteCountIEC(b int64) string {
    const unit = 1024
    if b < unit {
        return fmt.Sprintf("%d B", b)
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %ciB",
        float64(b)/float64(div), "KMGTPE"[exp])
}

