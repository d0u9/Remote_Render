package render

import (
    "fmt"
    "os/exec"
    "remote_render/pkg/public"
    cmd "remote_render/pkg/cmd"
    log "github.com/sirupsen/logrus"
)

type Render struct {
    bitrate     int
    speed       int
    user        string
    ip          string
    inC         chan *public.Task
    outC        chan *public.Task
    bin         string
    rCmd        *cmd.Cmd
    ctx         *public.QuitCtx
    isFree      bool
    readyC      chan interface{}
}

func New(user, ip string, bitrate, speed int, ctx *public.QuitCtx) (*Render, error) {
    bin, err := exec.LookPath("ssh")
    if err != nil {
        return nil, err
    }

    if bitrate == 0 && speed == 0 {
        return nil, fmt.Errorf("Speed and bitrate both are 0")
    }

    return &Render {
        bitrate:    bitrate,
        speed:      speed,
        user:       user,
        ip:         ip,
        inC:        make(chan *public.Task, 1),
        outC:       make(chan *public.Task, 1),
        bin:        bin,
        rCmd:       cmd.NewRemote(user, ip),
        ctx:        ctx,
        isFree:     true,
        readyC:     make(chan interface{}, 1),
    }, nil
}

func (r *Render) _Render(tsk *public.Task) (cmd.CmdCancelFunc, cmd.ResultChan) {
    return r.rCmd.Run("cp", tsk.RmtFile, tsk.RenderedFile)
}

func calCoe(speed float64) float64 {
    return (1 / speed)
}

func (r *Render) Render(tsk *public.Task) (cmd.CmdCancelFunc, cmd.ResultChan) {
    cmdOpts := []string {
        "ffmpeg",
        "-i",           tsk.RmtFile,
        "-an",
    }

    if r.speed != 0 {
        cmd := fmt.Sprintf("-filter:v \"copy,setpts=%.3f*PTS\"", calCoe(float64(r.speed)))
        cmdOpts = append(cmdOpts, cmd)
    }

    if r.bitrate != 0 {
        cmd := fmt.Sprintf("-b:v %dM -maxrate 100M", r.bitrate)
        cmdOpts = append(cmdOpts, cmd)
    }

    cmdOpts = append(cmdOpts, tsk.RenderedFile)

    log.Debugf("[%03d] [RENDER] COMMAND: %v\n", tsk.Idx, cmdOpts)

    return r.rCmd.Run(cmdOpts...)
}

func PrintQuitMsg() {
    log.Warnf("[XXX] [RENDER] Received DONE Signal\n")
}

func (r *Render) Watch() {
    var cmdResultChan   cmd.ResultChan
    var cmdCancel       cmd.CmdCancelFunc
    var curTask         *public.Task

    r.readyC <- true

    for {
        if r.isFree {
            select {
            case curTask = <- r.inC:
                log.Infof("[%03d] [RENDER] Srarting render: %s\n", curTask.Idx, curTask.SrcFile)
                r.isFree = false
                cmdCancel, cmdResultChan = r.Render(curTask)
            case <-r.ctx.Done():
                PrintQuitMsg()
                goto out
            }
        } else {
            select {
            case cmdResult := <- cmdResultChan:
                log.Infof("[%03d] [RENDER] Finished, new file: %s\n", curTask.Idx, curTask.SrcFile)
                if cmdResult.Err != nil {
                    curTask.Err = fmt.Errorf("%s", cmdResult.Output)
                }
                r.outC <- curTask
                r.isFree = true
                select {
                case r.readyC <- true:
                default:
                }
            case <-r.ctx.Done():
                PrintQuitMsg()
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

func (r *Render) Push(tsk *public.Task) {
    r.inC <- tsk
}

func (r *Render) ResultChan() chan *public.Task {
    return r.outC
}

func (r *Render) ReadyChan() chan interface{} {
    return r.readyC
}
