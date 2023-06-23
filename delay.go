package promise

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
	OnAll     DelayCond = iota // the default behavior if no conditions are passed
	OnSuccess DelayCond = iota
	OnError   DelayCond = iota
	OnPanic   DelayCond = iota
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

func getDelayFlags(modes []DelayCond) delayFlags {
	if len(modes) == 0 {
		return delayAllFlags
	}

	f := delayFlags{}
	for _, m := range modes {
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
