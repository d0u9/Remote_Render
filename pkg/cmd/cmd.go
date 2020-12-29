package cmd

import (
    "fmt"
    "os/exec"
    "context"
)

type CmdCancelFunc  context.CancelFunc
type ResultChan     chan CmdResult

type CmdResult struct {
    Output      string
    Err         error
}

type Cmd struct {
    isRemote        bool
    ip              string
    user            string
}

func NewLocal() *Cmd {
    return &Cmd {
        isRemote:       false,
        ip:             "",
        user:           "",
    }
}

func NewRemote(user, ip string) *Cmd {
    return &Cmd {
        isRemote:       true,
        ip:             ip,
        user:           user,
    }
}

func (c *Cmd) SyncRun(args ...string) (string, error) {
    _, resultChan := c.Run(args...)

    result := <- resultChan
    return result.Output, result.Err
}

func (c *Cmd) Run(args ...string) (CmdCancelFunc, ResultChan) {
    rstChan := make(ResultChan, 1)
    ctx, cancel := context.WithCancel(context.Background())

    if (c.isRemote) {
        go c.remoteRun(ctx, rstChan, args...)
    } else {
        go c.localRun(ctx, rstChan, args...)
    }

    return CmdCancelFunc(cancel), rstChan
}

func (c *Cmd) localRun(ctx context.Context, rstChan ResultChan, args ...string) {
    cmd := exec.CommandContext(ctx, args[0], args[1:]...)
    output, err := cmd.CombinedOutput()
    rstChan <- CmdResult{ Output: string(output), Err: err, }
}

func (c *Cmd) remoteRun(ctx context.Context, rstChan ResultChan, args ...string) {
    cmdStr := ""
    for _, arg := range args {
        cmdStr = fmt.Sprintf("%s %s", cmdStr, arg)
    }

    cmd := exec.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", c.user, c.ip), cmdStr)
    output, err := cmd.CombinedOutput()
    rstChan <- CmdResult{ Output: string(output), Err: err, }
}

type RemoteResult struct {
    Output      string
    Err         error
}

type RemoteCmd struct {
    ip          string
    user        string
    cancel      context.CancelFunc
    finishChan  chan RemoteResult
}

func New(user, ip string) *RemoteCmd {
    return &RemoteCmd {
        ip:     ip,
        user:   user,
        finishChan:     make(chan RemoteResult, 1),
    }
}

func (r *RemoteCmd) RunBackground(cmd string) {
    output, err := r.Run(cmd)

    r.finishChan <- RemoteResult{ Output: output, Err: err, }
}

func (r *RemoteCmd) Run(cmd string) (string, error) {
    ctx, cancel := context.WithCancel(context.Background())
    _cmd := exec.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", r.user, r.ip), cmd)
    r.cancel = cancel
    output, err := _cmd.CombinedOutput()
    return string(output), err
}

func (r *RemoteCmd) Cancel() {
    r.cancel()
}

func (r *RemoteCmd) ResultChan() chan RemoteResult {
    return r.finishChan
}
