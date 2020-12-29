package public

var (
    FinSigChan              chan FinSig
)

func init() {
    FinSigChan = make(chan FinSig)
}
