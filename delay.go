package promise

// DelayCond describes when the wait duration provided to [Delay] or [Group.Delay]
// takes effect, in respect with the provided [Result] value to the same call.
type DelayCond int

func (m DelayCond) String() string {
	switch m {
	case OnAll:
		return "OnAll"
	case OnSuccess:
		return "OnSuccess"
	case OnError:
		return "OnError"
	case OnPanic:
		return "OnPanic"
	default:
		return "<unknown condition>"
	}
}

// any values other than the listed below will be ignored
const (
	// OnAll means that [Delay] will wait on all [Result] values,
	// regardless of what [Result.State] returns.
	// This is the default behavior if no conditions are passed.
	OnAll DelayCond = iota

	// OnSuccess means that [Delay] will only wait on [Result] values
	// whose [Result.State] returns [Success].
	OnSuccess

	// OnError means that [Delay] will only wait on [Result] values
	// whose [Result.State] returns [Error].
	OnError

	// OnPanic means that [Delay] will only wait on [Result] values
	// whose [Result.State] returns [Panic].
	OnPanic
)

type delayFlags struct {
	onSuccess bool
	onError   bool
	onPanic   bool
}

var delayAllFlags = delayFlags{
	onSuccess: true,
	onError:   true,
	onPanic:   true,
}

func getDelayFlags(conds []DelayCond) delayFlags {
	if len(conds) == 0 {
		return delayAllFlags
	}

	f := delayFlags{}
	for _, m := range conds {
		switch m {
		case OnAll:
			f.onSuccess = true
			f.onError = true
			f.onPanic = true
		case OnSuccess:
			f.onSuccess = true
		case OnError:
			f.onError = true
		case OnPanic:
			f.onPanic = true
		}
	}
	return f
}
