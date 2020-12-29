package remote_copy

import (
    "fmt"
    "os/exec"
    "path/filepath"
    "remote_render/pkg/public"
    cmd "remote_render/pkg/cmd"
    log "github.com/sirupsen/logrus"
)

type RemoteCopy struct {
    user        string
    ip          string
    toRemote    bool
    inC         chan *public.Task
    outC        chan *public.Task
    bin         string
    lclCmd      *cmd.Cmd
    rmtCmd      *cmd.Cmd
    rCmd        *cmd.RemoteCmd
    ctx         *public.QuitCtx
    isFree      bool
    readyC      chan interface{}
}

func New(user, ip string, toRemote bool, ctx *public.QuitCtx) (*RemoteCopy, error) {
    bin, err := exec.LookPath("ssh")
    if err != nil {
        return nil, err
    }

    r := &RemoteCopy {
        user:       user,
        ip:         ip,
        toRemote:   toRemote,
        inC:        make(chan *public.Task, 1),
        outC:       make(chan *public.Task, 1),
        bin:        bin,
        rCmd:       cmd.New(user, ip),
        ctx:        ctx,
        lclCmd:     cmd.NewLocal(),
        rmtCmd:     cmd.NewRemote(user, ip),
        isFree:     true,
        readyC:     make(chan interface{}, 1),
    }

    return r, nil
}

func (r *RemoteCopy) Copy(inTask *public.Task) (cmd.CmdCancelFunc, cmd.ResultChan) {
    var cancel      cmd.CmdCancelFunc
    var resultChan  cmd.ResultChan

    if r.toRemote {
        // cancel, resultChan = r.lclCmd.Run("scp", inTask.SrcFile, fmt.Sprintf("%s@%s:%s", r.user, r.ip, inTask.RmtFile))
        cancel, resultChan = r.lclCmd.Run("rsync", "-avP", inTask.SrcFile, fmt.Sprintf("%s@%s:%s", r.user, r.ip, inTask.RmtFile))
    } else {
        cancel, resultChan = r.lclCmd.Run("scp", fmt.Sprintf("%s@%s:%s", r.user, r.ip, inTask.RenderedFile), inTask.DstFile)
    }

    return cancel, resultChan
}

func PrintQuitMsg(toRemote bool) {
    if toRemote {
        log.Warnf("[XXX] [COPY->] Received DONE Signal\n")
    } else {
        log.Warnf("[XXX] [<-COPY] Received DONE Signal\n")
    }
}

func (r *RemoteCopy) Watch() {
    var cmdResultChan   cmd.ResultChan
    var cmdCancel       cmd.CmdCancelFunc
    var curTask         *public.Task

    r.readyC <- true

    for {
        if r.isFree {
            select {
            case curTask = <- r.inC:
                r.isFree = false
                if r.toRemote {
                    log.Infof("[%03d] [COPY->] %s to %s\n", curTask.Idx, curTask.SrcFile, curTask.RmtFile)
                } else {
                    log.Infof("[%03d] [<-COPY] %s to %s\n", curTask.Idx, curTask.RenderedFile, curTask.DstFile)
                }
                cmdCancel, cmdResultChan = r.Copy(curTask)
            case <- r.ctx.Done():
                PrintQuitMsg(r.toRemote)
                goto out
            }
        } else {
            select {
            case cmdResult := <- cmdResultChan:
                if r.toRemote {
                    log.Infof("[%03d] [COPY->] DONE %s to %s\n", curTask.Idx, curTask.SrcFile, curTask.RmtFile)
                } else {
                    log.Infof("[%03d] [<-COPY] DONE %s to %s\n", curTask.Idx, curTask.RenderedFile, curTask.DstFile)
                }
                if cmdResult.Err != nil {
                    curTask.Err = fmt.Errorf("%s", cmdResult.Output)
                }
                r.outC <- curTask
                r.isFree = true
                select {
                case r.readyC <- true:
                default:
                }
            case <- r.ctx.Done():
                PrintQuitMsg(r.toRemote)
                cmdCancel()
                curTask.Err = fmt.Errorf("Task is canceled")
                r.outC <- curTask
                goto out
            }
        }
    }

out:
    r.ctx.Finish()
}

func (r *RemoteCopy) Push(tsk *public.Task) {
    r.inC <- tsk
}

func (r *RemoteCopy) ResultChan() chan *public.Task {
    return r.outC
}

func (r *RemoteCopy) ReadyChan() chan interface{} {
    return r.readyC
}

func (r *RemoteCopy) mkTargetDir(user, ip, dirname string, toRemote bool) error {
    if !filepath.IsAbs(dirname) {
        return fmt.Errorf("Target Directory is not absolute path: %s", dirname)
    }

    var resultChan  cmd.ResultChan

    if toRemote {
        _, resultChan = r.rmtCmd.Run("mkdir", "-p", dirname)
    } else {
        _, resultChan = r.lclCmd.Run("mkdir", "-p", dirname)
    }

    if result := <- resultChan; result.Err != nil {
        log.Fatalf("mkdir failed: %s\n", result.Output)
        return result.Err
    }

    return nil
}

