package app

import (
    "os"
    "fmt"
    // "sync"
    "sort"
    "time"
    "syscall"
    "strings"
    "os/signal"
    "io/ioutil"
    "path/filepath"
    "gopkg.in/yaml.v2"
    "github.com/fatih/color"
    "github.com/google/uuid"
    "github.com/spf13/cobra"
    "remote_render/pkg/public"
    "remote_render/pkg/render"
    cmd "remote_render/pkg/cmd"
    rcp "remote_render/pkg/remote_copy"
    log "github.com/sirupsen/logrus"

)

var (
    DftConfig       *config
    Red             func(a ...interface{}) string
    Magenta         func(a ...interface{}) string
    Yellow          func(a ...interface{}) string
    Cyan            func(a ...interface{}) string
    Green           func(a ...interface{}) string
)

func init() {
    homedir, err := os.UserHomeDir()
    if err != nil {
        log.Fatal("Cannot get user's home directory")
        os.Exit(1)
    }

    DftConfig = &config {
        Speed:          0,
        Bitrate:        0,
        FileDir:        "",
        NewDir:         "",
        ListFile:       "",
        User:           "",
        IP:             "",
        RstFile:        filepath.Join(homedir, "remote_render.output"),
        ErrFile:        filepath.Join(homedir, "remote_render.err"),
        RmtTmpDir:       "/tmp/speed_render",
        Debug:          false,
        CfgFile:        "",
        TestMode:       false,
    }

    Red     = color.New(color.FgRed).SprintFunc()
    Magenta = color.New(color.FgMagenta).SprintFunc()
    Yellow  = color.New(color.FgYellow).SprintFunc()
    Cyan    = color.New(color.FgCyan).SprintFunc()
    Green   = color.New(color.FgGreen).SprintFunc()
}

type config struct {
    Speed           int       `yaml:"speed,omitempty"`          // ffmpeg option, convert video speed to
    Bitrate         int       `yaml:"bitrate,omitempty"`        // ffmpeg option, convert video bitrate to
    FileDir         string    `yaml:"file_dir,omitempty"`       // the directory containing videos to be converted
    ListFile        string    `yaml:"list_file,omitempty"`      // the file containing list of video pathes to be converted
    NewDir          string    `yaml:"new_dir,omitempty"`        // save converted video to this directory
    User            string    `yaml:"user,omitempty"`           // username for ssh
    IP              string    `yaml:"ip,omitempty"`             // ip address for ssh
    RmtTmpDir       string    `yaml:"rmt_tmp_dir,omitempty"`    // remote temp dir
    RstFile         string    `yaml:"result_file,omitempty"`    // Successfully processed files
    ErrFile         string    `yaml:"error_file,omitempty"`     // failed files
    Debug           bool      `yaml:"debug,omitempty"`          // debug mode
    CfgFile         string                                      // config file
    TestMode        bool      `yaml:"test_mode,omitempty"`      // print files to be processed, and quit
}

type App struct {
    config      config
    cmd         *cobra.Command
    toRmt       *rcp.RemoteCopy
    toHost      *rcp.RemoteCopy
    render      *render.Render
    rCmd        *cmd.Cmd
    seqQuit     *public.SeqQuit
    tskCntChan  chan interface{}

    tasks       []*public.Task
    suc_tasks   []*public.Task

    rstFile     *os.File
    errFile     *os.File
    TaskNr      int
}

func New() *App {

    app := &App {
        seqQuit:        public.NewSeqQuit(),
        tskCntChan:     make(chan interface{}),

        tasks:          []*public.Task{},
    }

    app.cmd = &cobra.Command {
        Use:   filepath.Base(os.Args[0]),
        Short: "",
        Long: ``,
        Run: func(cmd *cobra.Command, args []string) {
            app.doit()
        },
    }

    app.cmdSetup()

    return app
}

func (app *App) ReadCfgFile() {
    localCfg := *DftConfig
    dat, err := ioutil.ReadFile(app.config.CfgFile)
    if err != nil {
        log.Fatalf("Read config file: %s, err: %v\n", app.config.CfgFile, err)
        os.Exit(1)
    }

    if err := yaml.Unmarshal(dat, &localCfg); err != nil {
        log.Fatalf("Unmarshal config failed: %v\n", err)
        os.Exit(1)
    }

    if app.config.Speed     == DftConfig.Speed {
        app.config.Speed    = localCfg.Speed
    }

    if app.config.Bitrate   == DftConfig.Bitrate {
        app.config.Bitrate  = localCfg.Bitrate
    }
    if app.config.FileDir   == DftConfig.FileDir {
        app.config.FileDir  = localCfg.FileDir
    }
    if app.config.NewDir    == DftConfig.NewDir {
        app.config.NewDir   = localCfg.NewDir
    }
    if app.config.ListFile  == DftConfig.ListFile {
        app.config.ListFile = localCfg.ListFile
    }
    if app.config.User      == DftConfig.User {
        app.config.User     = localCfg.User
    }
    if app.config.IP        == DftConfig.IP {
        app.config.IP       = localCfg.IP
    }
    if app.config.RstFile   == DftConfig.RstFile {
        app.config.RstFile  = localCfg.RstFile
    }
    if app.config.ErrFile   == DftConfig.ErrFile {
        app.config.ErrFile  = localCfg.ErrFile
    }
    if app.config.RmtTmpDir == DftConfig.RmtTmpDir {
        app.config.RmtTmpDir= localCfg.RmtTmpDir
    }
    if app.config.Debug     == DftConfig.Debug {
        app.config.Debug    = localCfg.Debug
    }
    if app.config.TestMode  == DftConfig.TestMode {
        app.config.TestMode = localCfg.TestMode
    }
}

func (app *App) Init() {
    var quitCtx *public.QuitCtx

    if app.config.Debug {
        log.SetLevel(log.DebugLevel)
    }

    if app.config.CfgFile != "" {
        app.ReadCfgFile()
    }


    quitCtx = public.NewQuitCtx(10)
    toRmt, err := rcp.New(app.config.User, app.config.IP, true, quitCtx)
    if err != nil {
        log.Fatal("Cannot new remote copy instance")
        os.Exit(1)
    }
    app.toRmt = toRmt
    app.seqQuit.AddSeq(quitCtx)

    quitCtx = public.NewQuitCtx(20)
    render, err := render.New(app.config.User, app.config.IP, app.config.Bitrate, app.config.Speed, quitCtx)
    if err != nil {
        log.Fatal("Cannot create new render")
        os.Exit(1)
    }
    app.render = render
    app.seqQuit.AddSeq(quitCtx)

    quitCtx = public.NewQuitCtx(30)
    toHost, err := rcp.New(app.config.User, app.config.IP, false, quitCtx)
    if err != nil {
        log.Fatal("Cannot new remote copy instance")
        os.Exit(1)
    }
    app.toHost = toHost
    app.seqQuit.AddSeq(quitCtx)


    rCmd := cmd.NewRemote(app.config.User, app.config.IP)
    app.rCmd = rCmd

    rstFile, err := os.OpenFile(app.config.RstFile, os.O_WRONLY | os.O_TRUNC | os.O_CREATE, 0755)
    if err != nil {
        log.Fatalf("Cannot create result file: %s, err: %v\n", app.config.RstFile, err)
        os.Exit(1)
    }
    app.rstFile = rstFile

    errFile, err := os.OpenFile(app.config.ErrFile, os.O_WRONLY | os.O_TRUNC | os.O_CREATE, 0755)
    if err != nil {
        log.Fatalf("Cannot create error file: %s, err: %v\n", app.config.ErrFile, err)
        os.Exit(1)
    }
    app.errFile = errFile
}

func (app *App) ValidateConfig() {
    if app.config.FileDir != "" && app.config.ListFile != "" {
        log.Fatal("File dir or file list cannot use simulatenously.")
        os.Exit(1)
    }

    if app.config.User == "" || app.config.IP == "" {
        log.Fatal("username or ip address not assigned.")
        os.Exit(1)
    }

    if app.config.NewDir != "" {
        if !filepath.IsAbs(app.config.NewDir) {
            log.Fatal("New dir %s must be absolute path", app.config.NewDir)
            os.Exit(1)
        }

        if finfo, err := os.Stat(app.config.NewDir); err != nil {
            log.Fatalf("New dir is wrong: %v", err)
            os.Exit(1)
        } else if !finfo.IsDir() {
            log.Fatal("New dir %s is not a directory\n", app.config.NewDir)
            os.Exit(1)
        }
    }

    bitrate := app.config.Bitrate
    if bitrate != 0 && (bitrate < 10 || bitrate > 100) {
        log.Fatalf("Bitrate = %d is invalid, must be >= 10 and <= 100", bitrate)
        os.Exit(1)
    }

    speed := app.config.Speed
    if speed != 0 && (speed < 0 || speed > 100) {
        log.Fatalf("Speed = %d is invalid, must be >= 0 and <= 100", speed)
        os.Exit(1)
    }

    if app.config.Speed == 0 && app.config.Bitrate == 0 {
        log.Fatalf("Speed == 0 && bitrate == 0, nothinig to do with")
        os.Exit(1)
    }
}

func (app *App) EnumerateFilesFromFileDir() {
    if err := os.Chdir(app.config.FileDir); err != nil {
        log.Fatal("Cannot change working directory to %s, error %v\n", app.config.FileDir, err)
        os.Exit(1)
    }

    walkFunc := func(path string, info os.FileInfo, inerr error) error {
        if inerr != nil {
            log.Warn("prevent panic by handling failure accessing a path %q: %v\n", path, inerr)
            return inerr
        }

        if info.IsDir() {
            return nil
        }

        if !info.Mode().IsRegular() && (info.Mode() & os.ModeSymlink == 0) {
            return nil
        }

        var relFile, srcFile, srcDir string
        var err error = nil
        var fsize int64 = 0

        srcFile = path
        extUppered := strings.ToUpper(filepath.Ext(path))
        validExts := []string {
            ".MP4",
            ".MOV",
        }

        for _, vext := range validExts {
            if extUppered == vext {
                goto go_on
            }
        }

        err = fmt.Errorf("Wrong extension, %s", extUppered)
        goto add_task

go_on:
        if info.Mode() & os.ModeSymlink != 0 {
            relFile, err = filepath.EvalSymlinks(path)
            if err != nil {
                goto add_task
            }
            srcFile, err = filepath.Abs(relFile)
            if err != nil {
                goto add_task
            }
        } else {
            srcDir = app.config.FileDir
            srcFile = filepath.Join(srcDir, path)
        }

        if finfo, ferr := os.Stat(srcFile); ferr != nil {
            log.Errorf("Get file stat failed: %s, err: %v\n", srcFile, ferr)
            return nil
        } else {
            fsize = finfo.Size()
        }

        // TODO: Test if this file is a video file


add_task:
        app.tasks = append(app.tasks, &public.Task {
            SrcFile:    srcFile,
            Size:       fsize,
            Err:        err,
        })

        return nil
    }

    if err := filepath.Walk(".", walkFunc); err != nil {
        log.Fatal("Errro walking the path: %s: %v\n", app.config.FileDir, err)
        os.Exit(1)
    }

    sort.Slice(app.tasks, func(i, j int) bool {
        return app.tasks[i].Size < app.tasks[j].Size
    })

}

func (app *App) FilterErrorTasks(tsk *public.Task, stg public.Stage) (*public.Task, error) {
    if tsk.Err != nil {
        log.Infof("[%3d/%3d] [%s], Src: %s, Reason: %s\n", tsk.Idx, tsk.AllNr, Red("ERROR"), tsk.SrcFile, Red(tsk.Err.Error()))
        tsk.DieOn = stg
        app.tskCntChan <- true
    }
    return tsk, tsk.Err
}

func PrintQuitMsg(con string) {
    log.Warnf("[XXX] [%s] Received DONE signal\n", con);
}

func (app *App) S1Core(quitCtx *public.QuitCtx) {
    var err     error
    var curTask *public.Task
    waitingNewData := true

    for {
        if waitingNewData {
            select {
            case s1 := <- app.toRmt.ResultChan():
                if curTask, err = app.FilterErrorTasks(s1, public.STG_COPT_TO); err == nil {
                    log.Debugf("[%3d/%3d] [S1] Get tmp file done: %s\n", curTask.Idx, curTask.AllNr, curTask.RmtFile)
                    waitingNewData = false
                }
            case <- quitCtx.Done():
                PrintQuitMsg("S1")
                goto out
            }
        } else {
            select {
            case <-app.render.ReadyChan():
                log.Debugf("[%3d/%3d] [S1] PUSH file to render: %s\n", curTask.Idx, curTask.AllNr, curTask.RmtFile)
                app.render.Push(curTask)
                waitingNewData = true
            case <- quitCtx.Done():
                PrintQuitMsg("S1")
                goto out
            }
        }
    }

out:
    quitCtx.Finish()
}

func (app *App) S2Core(quitCtx *public.QuitCtx) {
    var err     error
    var curTask *public.Task
    waitingNewData := true

    for {
        if waitingNewData {
            select {
            case s2 := <- app.render.ResultChan():
                if curTask, err = app.FilterErrorTasks(s2, public.STG_RENDER); err == nil {
                    log.Debugf("[%3d/%3d] [S2] Get rendered file done: %s\n", curTask.Idx, curTask.AllNr, curTask.RenderedFile)
                    waitingNewData = false
                }
            case <- quitCtx.Done():
                goto out
            }
        } else {
            select {
            case <-app.toHost.ReadyChan():
                log.Debugf("[%3d/%3d] [S2] Push rendered file: %s\n", curTask.Idx, curTask.AllNr, curTask.RenderedFile)
                app.toHost.Push(curTask)
                waitingNewData = true
            case <- quitCtx.Done():
                goto out
            }
        }
    }

out:
    quitCtx.Finish()
}

func (app *App) S3Core(quitCtx *public.QuitCtx) {
    var err     error
    var curTask *public.Task
    var cmdResultChan   cmd.ResultChan
    var cmdCancel       cmd.CmdCancelFunc
    waitingNewData := true

    for {
        if waitingNewData {
            select {
            case s3 := <- app.toHost.ResultChan():
                if curTask, err = app.FilterErrorTasks(s3, public.STG_COPY_FROM); err == nil {
                    log.Debugf("[%3d/%3d] [S3] Get rendered file: %s\n", curTask.Idx, curTask.AllNr, curTask.DstFile)
                    waitingNewData = false
                    cmdCancel, cmdResultChan = app.rCmd.Run("rm", "-fr", s3.RmtFile, s3.RenderedFile)
                }
            case <- quitCtx.Done():
                goto out
            }
        } else {
            select {
            case result := <- cmdResultChan:
                if result.Err != nil {
                    curTask.Err = fmt.Errorf("%s", result.Output)
                } else {
                    curTask.DieOn = public.STG_DONE
                    app.suc_tasks = append(app.suc_tasks, curTask)
                }
                log.Debugf("[%3d/%3d] [S3] Removed rendered file: %s\n", curTask.Idx, curTask.AllNr, curTask.DstFile)
                size := public.ByteCountIEC(curTask.Size)
                log.Infof("[%3d/%3d] [S3] [%s], Size: %s, SRC: %s, DST: %s\n", curTask.Idx, curTask.AllNr, Red("DONE"), Yellow(size), Cyan(curTask.SrcFile), Green(curTask.DstFile))
                app.tskCntChan <- true
                waitingNewData = true
            case <- quitCtx.Done():
                cmdCancel()
                goto out
            }
        }
    }

out:
    quitCtx.Finish()
}

func (app *App) RunCore() {
    // c := make(chan public.Task)
    var quitCtx *public.QuitCtx

    quitCtx = public.NewQuitCtx(11)
    app.seqQuit.AddSeq(quitCtx)
    go app.S1Core(quitCtx)

    quitCtx = public.NewQuitCtx(21)
    app.seqQuit.AddSeq(quitCtx)
    go app.S2Core(quitCtx)

    quitCtx = public.NewQuitCtx(31)
    app.seqQuit.AddSeq(quitCtx)
    go app.S3Core(quitCtx)
}

func (app *App) EnumerateFilesFromListFile() {

}

func (app *App) DistributeTasks() {
    time.Sleep(time.Second)

    quitCtx := public.NewQuitCtx(0)
    app.seqQuit.AddSeq(quitCtx)

    var i int
    var tsk *public.Task
    canSend := false
    for i, tsk = range app.tasks {
        if canSend {
            if tsk.Err != nil {
                continue
            }
            size := public.ByteCountIEC(tsk.Size)
            tskInfo := fmt.Sprintf("Size: %s, SRC: %s, DST: %s", Yellow(size), Cyan(tsk.SrcFile), Green(tsk.DstFile))
            log.Infof("[%3d/%3d] [%s] Push Task: %s\n", tsk.Idx, tsk.AllNr, Magenta("DISTRIBUTOR"), tskInfo)
            app.toRmt.Push(tsk)
            canSend = false
        }

        select {
        case <- quitCtx.Done():
            log.Warnf("[XXX] [DISTRIBUTOR] DistributeTasks() Stops\n")
            goto out
        case <- app.toRmt.ReadyChan():
            canSend = true

        }
    }


out:
    quitCtx.Finish()
    log.Infof("Only %d files processed\n", i + 1)
}

func (app *App) AssignTmpFile() {

    log.Infof("------------------- File List (BELOW) -------------------\n")
    allNr := 0
    for _, tsk := range app.tasks {
        if tsk.Err != nil {
            continue
        }
        ext := filepath.Ext(tsk.SrcFile)
        nameNoExt := strings.TrimSuffix(filepath.Base(tsk.SrcFile), ext)
        randomName := uuid.New().String()
        rmtFname := fmt.Sprintf("%s%s", randomName, ext)
        renderedFile := fmt.Sprintf("%s_rendered%s", randomName, ext)
        dstDir := app.config.NewDir
        if dstDir == "" {
            dstDir = filepath.Dir(tsk.SrcFile)
        }

        tsk.DieOn = public.STG_INIT
        tsk.RmtFile = filepath.Join(app.config.RmtTmpDir, rmtFname)
        tsk.RenderedFile = filepath.Join(app.config.RmtTmpDir, renderedFile)
        suffix := ""
        if app.config.Speed != 0 {
            suffix = fmt.Sprintf("%s_%dX", suffix, app.config.Speed)
        }

        if app.config.Bitrate != 0 {
            suffix = fmt.Sprintf("%s_%dBPS", suffix, app.config.Bitrate)
        }

        tsk.DstFile = filepath.Join(dstDir, fmt.Sprintf("%s%s%s", nameNoExt, suffix, ext))
        allNr = allNr + 1
        tsk.Idx = allNr
        log.Infof("- SRC: [%s] %s\n", Yellow(public.ByteCountIEC(tsk.Size)), tsk.SrcFile)
        log.Infof("+ DST: [%s] %s\n", "", tsk.DstFile)
    }
    log.Infof("------------------- File List (ABOVE) -------------------\n")
    app.TaskNr = allNr

    log.Infof("------------------- %s (BELOW) -------------------\n", Red("Error Files"))
    for _, tsk := range app.tasks {
        if tsk.Err == nil {
            tsk.AllNr = allNr
            continue
        }
        log.Errorf("%s error: %v\n", tsk.SrcFile, tsk.Err)
    }
    log.Infof("------------------- %s (ABOVE) -------------------\n", Red("Error Files"))

    if app.config.TestMode {
        log.Warnf("In Test Mode, quiting...\n")
        os.Exit(0)
    }
}

func (app *App) Prepare() {
    if app.config.NewDir != "" {
        if err := os.MkdirAll(app.config.NewDir, 0666); err != nil {
            log.Fatal("Cannot create local dir: %s, err: %v\n", app.config.NewDir, err)
            os.Exit(1)
        }
    }

    if _, err := app.rCmd.SyncRun("mkdir", "-p", app.config.RmtTmpDir); err != nil {
        log.Fatal("Cannot create remote dir: %s, err: %v\n", app.config.RmtTmpDir, err)
        os.Exit(1)
    }
}

func (app *App) doit() {
    app.Init()
    app.ValidateConfig()

    log.Infof("Config: %+v\n", app.config)

    if app.config.FileDir != "" {
        app.EnumerateFilesFromFileDir()
    } else {
        app.EnumerateFilesFromListFile()
    }

    app.AssignTmpFile()
    app.Prepare()

    log.Debugf("Tasks: %+v\n", app.tasks)

    go app.RunCore()
    go app.toRmt.Watch()
    go app.render.Watch()
    go app.toHost.Watch()
    go app.DistributeTasks()

    i := 0
    quiting := false
    allDone := make(chan interface{})
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

    go func() {
        for {
            select {
            case <-app.tskCntChan:
                i = i + 1
                if i >= app.TaskNr {
                    log.Warn("All Done")
                    allDone <- true
                }
            }
        }
    }()

    select {
    case <- allDone:
        goto out
    case s := <- sigChan:
        if !quiting {
            log.Errorf("Receive signal, %v\n", s)
            quiting = true
            app.seqQuit.Quit()
            goto out
        }
    }

out:
    app.DumpResult()
    log.Debugf("Tasks: %+v\n", app.tasks)
}

func (app *App) DumpResult() {
    suc_tasks := []*public.Task{}
    err_tasks := []*public.Task{}
    for _, tsk := range app.tasks {
        if tsk.Err == nil && tsk.DieOn == public.STG_DONE {
            suc_tasks = append(suc_tasks, tsk)
        } else {
            if tsk.Err == nil {
                tsk.Err = fmt.Errorf("Task is canceled")
            }
            err_tasks = append(err_tasks, tsk)
        }
    }

    for _, tsk := range suc_tasks {
        str1 := fmt.Sprintf("- %s\n", tsk.SrcFile)
        app.rstFile.WriteString(str1)
        str2 := fmt.Sprintf("+ %s\n", tsk.DstFile)
        app.rstFile.WriteString(str2)
    }
    app.rstFile.Close()

    for _, tsk := range err_tasks {
        str1 := fmt.Sprintf("+++FILE+++ %s\n", tsk.SrcFile)
        app.errFile.WriteString(str1)
        str2 := fmt.Sprintf("%v\n", tsk.Err)
        app.errFile.WriteString(str2)
    }
    app.errFile.Close()
}

func (app *App) Run() {
    app.cmd.Execute()
}

func (app *App) cmdSetup() {

    flags := app.cmd.PersistentFlags()
    flags.IntVarP   (&app.config.Speed,     "speed",    "s",    DftConfig.Speed,    "new view speed")
    flags.IntVarP   (&app.config.Bitrate,   "bitrate",  "b",    DftConfig.Bitrate,  "new bitrate")
    flags.StringVarP(&app.config.FileDir,   "dir",      "d",    DftConfig.FileDir,  "dir containing files")
    flags.StringVarP(&app.config.NewDir,    "output",   "o",    DftConfig.NewDir,   "save new file in this dir")
    flags.StringVarP(&app.config.ListFile,  "file",     "f",    DftConfig.ListFile, "file conatining files")
    flags.StringVarP(&app.config.User,      "user",     "u",    DftConfig.User,     "username for ssh")
    flags.StringVarP(&app.config.IP,        "ip",       "a",    DftConfig.IP,       "address of remote render")
    flags.StringVarP(&app.config.RstFile,   "result",   "r",    DftConfig.RstFile,  "file to save success files")
    flags.StringVarP(&app.config.ErrFile,   "error",    "e",    DftConfig.ErrFile,  "file to save err files")
    flags.StringVarP(&app.config.RmtTmpDir, "tmp",      "t",    DftConfig.RmtTmpDir,"tmp dir on remote host")
    flags.BoolVar   (&app.config.Debug,     "debug",            DftConfig.Debug,    "enable debug mode")
    flags.StringVarP(&app.config.CfgFile,   "config",   "c",    DftConfig.CfgFile,  "config file")
    flags.BoolVarP  (&app.config.TestMode,  "test",     "q",    DftConfig.TestMode, "list files to process")
}
